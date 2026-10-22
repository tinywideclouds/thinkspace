import { Injectable } from '@angular/core';
import { Observable, Subject } from 'rxjs';
import { WSEvent, WSEventSchema } from '@org/llm-core-protos';
import { create } from '@bufbuild/protobuf';
import mockGraph from './mock-graph.json';

interface MockScenarioStep {
  offsetMs: number;
  payload: any;
}

@Injectable({ providedIn: 'root' })
export class MockTransportService {
  private inbound$ = new Subject<WSEvent>();
  private allEvents = [...mockGraph.events]; 

  public connect(url: string): Observable<WSEvent> {
    console.log('🔌 [MOCK] Connected. Send "/debug [name]" to trigger a scenario.');
    setTimeout(() => this.handleLoadHistory({ limit: 20, cursor: '' }), 150);
    return this.inbound$.asObservable();
  }

  public send(message: WSEvent): void {
    if (message.payload.case === 'loadHistory') {
      this.handleLoadHistory(message.payload.value);
    } else if (message.payload.case === 'submitPrompt') {
      const text = message.payload.value.text.trim();
      if (text.startsWith('/debug ') || text.startsWith('/d ')) {
        let scenarioName = text.replace('/d ', '');
        scenarioName = text.replace('/debug ', '');
        this.playScenario(scenarioName);
      } else {
        this.handleFakeTurn(text);
      }
    }
  }

  public disconnect(): void {
    this.inbound$.complete();
  }

  private handleLoadHistory(payload: { cursor?: string, limit: number }): void {
    setTimeout(() => {
      let endIndex = this.allEvents.length;
      if (payload.cursor) {
        const cursorIdx = this.allEvents.findIndex(e => e.id === payload.cursor);
        if (cursorIdx !== -1) endIndex = cursorIdx; 
      }
      const startIndex = Math.max(0, endIndex - payload.limit);
      const page = this.allEvents.slice(startIndex, endIndex);
      
      const hasMore = startIndex > 0;
      const nextCursor = page.length > 0 ? page[0].id : (payload.cursor || '');

      this.inbound$.next(create(WSEventSchema, {
        payload: {
          case: 'syncHistory',
          value: {
            recentEvents: page,
            digests: payload.cursor ? {} : mockGraph.digests, 
            focus: payload.cursor ? undefined : mockGraph.focus,
            nextCursor,
            hasMore
          }
        }
      }));
    }, 300);
  }

  private playScenario(name: string): void {
    const scenarioData = (mockGraph.scenarios as Record<string, MockScenarioStep[]>)[name];
    if (!scenarioData || !Array.isArray(scenarioData)) {
      console.warn(`[MOCK] Scenario "${name}" not found in mock-graph.scenarios`);
      return;
    }

    console.log(`🎬 [MOCK] Playing scenario: ${name}`);
    for (const step of scenarioData) {
      setTimeout(() => {
        this.inbound$.next(create(WSEventSchema, { payload: step.payload }));
      }, step.offsetMs);
    }
  }

  private handleFakeTurn(promptText: string): void {
    const flowId = crypto.randomUUID();
    setTimeout(() => {
      this.inbound$.next(create(WSEventSchema, {
        payload: { case: 'telemetryEvent', value: { channel: 'manager', event: { id: crypto.randomUUID(), type: 'flow_start', timestamp: new Date().toISOString(), message: '', summary: 'Synthesizing response...', payloadJson: JSON.stringify({ flow_id: flowId, agent_count: 1 }) } } }
      }));
    }, 200);
    setTimeout(() => {
      this.inbound$.next(create(WSEventSchema, {
        payload: { case: 'chatStream', value: { text: `Mock response to: "${promptText}"` } }
      }));
    }, 800);
    setTimeout(() => {
      this.inbound$.next(create(WSEventSchema, {
        payload: { case: 'telemetryEvent', value: { channel: 'manager', event: { id: crypto.randomUUID(), type: 'flow_complete', timestamp: new Date().toISOString(), message: '', summary: '', payloadJson: JSON.stringify({ flow_id: flowId, agent_index: 1, passed: true }) } } }
      }));
    }, 1200);
  }
}