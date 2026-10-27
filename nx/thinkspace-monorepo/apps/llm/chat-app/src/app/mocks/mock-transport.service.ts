import { Injectable, inject } from '@angular/core';
import { Observable, Subject } from 'rxjs';
import { WSEvent, WSEventSchema } from '@org/llm-core-protos';
import { create } from '@bufbuild/protobuf';
import { UiSettingsService } from '@org/llm-state-chat';
import mockGraph from './mock-graph.json';

interface MockScenarioStep {
  offsetMs: number;
  payload?: any;
  latencyMs?: number;
  uiSettings?: {
    pagination: {
      size: number;
      prefetchIndex: number;
    };
  };
}

@Injectable({ providedIn: 'root' })
export class MockTransportService {
  private inbound$ = new Subject<WSEvent>();
  private allEvents = [...mockGraph.events]; 
  private uiSettings = inject(UiSettingsService);
  private currentLatencyMs = 500;

  public connect(url: string): Observable<WSEvent> {
    console.log('🔌 [MOCK] Connected. Send "/debug [name]" to trigger a scenario.');
    setTimeout(() => this.handleLoadHistory({ limit: this.uiSettings.pagination().size, cursor: '' }), 150);
    return this.inbound$.asObservable();
  }

  public send(message: WSEvent): void {
    if (message.payload.case === 'loadHistory') {
      this.showMockSnackbar(`📡 Pagination Triggered: Loading ${message.payload.value.limit} items`);
      this.handleLoadHistory(message.payload.value); 
    } else if (message.payload.case === 'submitPrompt') {
      const text = message.payload.value.text.trim();
      if (text.startsWith('/debug ') || text.startsWith('/d ')) {
        const scenarioName = text.replace('/d ', '').replace('/debug ', '');
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
    }, this.currentLatencyMs); 
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
        if (step.latencyMs !== undefined) {
          this.currentLatencyMs = step.latencyMs;
        }

        if (step.uiSettings) {
          if (step.uiSettings.pagination !== undefined) {
            this.uiSettings.pagination.set(step.uiSettings.pagination);
          }
          if (!step.payload) return;
        }

        if (step.payload?.case === 'syncHistory') {
          this.allEvents = step.payload.value.recentEvents;
          this.handleLoadHistory({ limit: this.uiSettings.pagination().size, cursor: '' });
        } else if (step.payload) {
          this.inbound$.next(create(WSEventSchema, { payload: step.payload }));
        }
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

  private showMockSnackbar(message: string): void {
    const toast = document.createElement('div');
    toast.textContent = message;
    toast.style.cssText = `
      position: fixed;
      bottom: 80px;
      left: 50%;
      transform: translateX(-50%);
      background-color: #1e1e1e;
      color: #4ade80;
      padding: 10px 20px;
      border-radius: 8px;
      z-index: 99999;
      font-family: monospace;
      font-size: 12px;
      font-weight: bold;
      box-shadow: 0 4px 12px rgba(0,0,0,0.3);
      pointer-events: none;
      transition: opacity 0.3s;
    `;
    
    document.body.appendChild(toast);
    
    setTimeout(() => {
      toast.style.opacity = '0';
      setTimeout(() => toast.remove(), 300);
    }, 2500);
  }
}