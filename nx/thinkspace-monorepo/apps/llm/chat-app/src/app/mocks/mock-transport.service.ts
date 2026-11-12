import { Injectable, inject } from '@angular/core';
import { Observable, Subject } from 'rxjs';
import { WSEvent, WSEventSchema, GraphEventSchema } from '@org/llm-core-protos';
import { create, fromJson, JsonValue } from '@bufbuild/protobuf';
import { UiSettingsService } from '@org/llm-state-chat';
import { LoggerService } from '@org/core-logger';
import rawMockGraph from './mock-graph.json';

export interface MockScenarioStep {
  offsetMs: number;
  payload?: JsonValue;
  latencyMs?: number;
  awaitAction?: string;
  uiSettings?: {
    pagination: {
      size: number;
      prefetchIndex: number;
    };
  };
}

export interface MockGraphData {
  events: JsonValue[];
  digests: Record<string, unknown>;
  focus: unknown;
  scenarios: Record<string, MockScenarioStep[]>;
  files: Record<string, string>;
  subgraphs: Record<string, JsonValue[]>;
}

const mockGraph = rawMockGraph as unknown as MockGraphData;

@Injectable({ providedIn: 'root' })
export class MockTransportService {
  private logger = inject(LoggerService);
  private inbound$ = new Subject<WSEvent>();
  private allEvents: JsonValue[] = [...mockGraph.events];
  private uiSettings = inject(UiSettingsService);
  private currentLatencyMs = 500;
  
  private pendingActionResolver: ((value: unknown) => void) | null = null;
  private expectedAction: string | null = null;

  constructor() {
    this.validateMockGraph();
  }

  private validateMockGraph(): void {
    let hasErrors = false;
    this.logger.info('MockTransport', 'Validating mock-graph.json structure...');

    this.allEvents.forEach((event, index) => {
      const ev = event as Record<string, unknown>;
      if (ev['type'] || (ev['payload'] && (ev['payload'] as Record<string, unknown>)['case'])) {
        this.logger.error('MockTransport', `[Invalid History Event] Index ${index} (ID: ${ev['id']}): Uses legacy format. Expected standard Protobuf JSON.`);
        hasErrors = true;
      }
      if (ev['branchId']) {
         this.logger.error('MockTransport', `[Invalid History Event] Index ${index} (ID: ${ev['id']}): Contains deprecated branchId field.`);
         hasErrors = true;
      }
    });

    for (const [scenarioName, steps] of Object.entries(mockGraph.scenarios)) {
      steps.forEach((step, index) => {
        if (step.payload) {
          const payload = step.payload as Record<string, unknown>;
          if (payload['case'] || (payload['payload'] && (payload['payload'] as Record<string, unknown>)['case'])) {
            this.logger.error('MockTransport', `[Invalid Scenario Step] Scenario "${scenarioName}" Step ${index}: Uses legacy "case/value" format.`);
            hasErrors = true;
          }
          
          const streamed = payload['streamedGraphEvent'] as Record<string, unknown>;
          if (streamed?.['event'] && (streamed['event'] as Record<string, unknown>)['type']) {
            this.logger.error('MockTransport', `[Invalid Scenario Step] Scenario "${scenarioName}" Step ${index}: Nested GraphEvent uses legacy format.`);
            hasErrors = true;
          }
          if (streamed?.['event'] && (streamed['event'] as Record<string, unknown>)['branchId']) {
            this.logger.error('MockTransport', `[Invalid Scenario Step] Scenario "${scenarioName}" Step ${index}: Nested GraphEvent contains deprecated branchId.`);
            hasErrors = true;
          }
        }
      });
    }

    if (hasErrors) {
      this.logger.error('MockTransport', 'Mock graph validation failed! The UI may drop messages. Update your JSON data.');
    } else {
      this.logger.success('MockTransport', 'Mock graph is valid Protobuf JSON.');
    }
  }

  public connect(url: string): Observable<WSEvent> {
    this.logger.info('MockTransport', '🔌 Connected. Send "/debug [name]" to trigger a scenario.');
    setTimeout(() => this.handleLoadHistory({ limit: this.uiSettings.pagination().size, cursor: '' }), 150);
    return this.inbound$.asObservable();
  }

  public send(message: WSEvent): void {
    if (!message.payload || message.payload.case === undefined) return;
    
    const actionCase = message.payload.case;

    if (this.pendingActionResolver && actionCase === this.expectedAction) {
      this.pendingActionResolver(null);
      return; 
    }

    if (actionCase === 'loadHistory') {
      this.showMockSnackbar(`📡 Pagination Triggered: Loading ${message.payload.value.limit} items`);
      this.handleLoadHistory(message.payload.value);
    } else if (actionCase === 'loadSubGraph') {
      this.showMockSnackbar(`🕸️ DAG Projection Triggered: Loading SubGraph ${message.payload.value.graphId}`);
      this.handleLoadSubGraph(message.payload.value);
    } else if (actionCase === 'submitPrompt') {
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

  private handleLoadHistory(payload: { cursor?: string; limit: number }): void {
    setTimeout(() => {
      let endIndex = this.allEvents.length;
      if (payload.cursor) {
        const cursorIdx = this.allEvents.findIndex(e => (e as Record<string, unknown>)['id'] === payload.cursor);
        if (cursorIdx !== -1) endIndex = cursorIdx;
      }

      const startIndex = Math.max(0, endIndex - payload.limit);
      const page = this.allEvents.slice(startIndex, endIndex);

      const hasMore = startIndex > 0;
      const nextCursor = page.length > 0 ? ((page[0] as Record<string, unknown>)['id'] as string) : (payload.cursor || '');

      try {
        const mappedEvents = page.map(e => fromJson(GraphEventSchema, e));

        this.inbound$.next(create(WSEventSchema, {
          payload: {
            case: 'syncHistory',
            value: {
              recentEvents: mappedEvents,
              digests: payload.cursor ? {} : mockGraph.digests as Record<string, any>,
              focus: payload.cursor ? undefined : mockGraph.focus as any,
              nextCursor,
              hasMore,
              spaceId: 'project-alpha',
              chatId: 'chat-1',
              projectionId: 'main'
            }
          }
        }));
      } catch (err) {
        this.logger.error('MockTransport', 'FATAL: Failed to construct syncHistory payload!', err);
      }
    }, this.currentLatencyMs);
  }

  private handleLoadSubGraph(payload: { spaceId: string; chatId: string; graphId: string }): void {
    setTimeout(() => {
      const graphData = mockGraph.subgraphs[payload.graphId] || [];

      try {
        const mappedEvents = graphData.map(e => fromJson(GraphEventSchema, e));

        this.inbound$.next(create(WSEventSchema, {
          payload: {
            case: 'syncSubGraph',
            value: {
              events: mappedEvents,
              spaceId: payload.spaceId,
              chatId: payload.chatId,
              projectionId: payload.graphId
            }
          }
        }));
      } catch (err) {
        this.logger.error('MockTransport', 'FATAL: Failed to construct syncSubGraph payload!', err);
      }
    }, this.currentLatencyMs);
  }

  private async playScenario(name: string): Promise<void> {
    const scenarioData = mockGraph.scenarios[name];
    if (!scenarioData || !Array.isArray(scenarioData)) {
      this.logger.warn('MockTransport', `Scenario "${name}" not found in mock-graph.scenarios`);
      return;
    }

    this.logger.info('MockTransport', `🎬 Playing scenario: ${name}`);

    for (const step of scenarioData) {
      if (step.offsetMs) {
        await new Promise(resolve => setTimeout(resolve, step.offsetMs));
      }

      if (step.latencyMs !== undefined) {
        this.currentLatencyMs = step.latencyMs;
      }

      if (step.uiSettings?.pagination !== undefined) {
        this.uiSettings.pagination.set(step.uiSettings.pagination);
      }

      if (step.payload) {
        try {
          const payloadRecord = step.payload as Record<string, unknown>;
          if (payloadRecord['syncHistory']) {
            this.allEvents = (payloadRecord['syncHistory'] as Record<string, unknown>)['recentEvents'] as JsonValue[];
            this.handleLoadHistory({ limit: this.uiSettings.pagination().size, cursor: '' });
          } else {
            if (payloadRecord['streamedGraphEvent']) {
               const sge = payloadRecord['streamedGraphEvent'] as any;
               sge.spaceId = sge.spaceId || 'project-alpha';
               sge.chatId = sge.chatId || 'chat-1';
               sge.projectionId = sge.projectionId || 'main';
            }
            this.logger.info('MockTransport', `[DEBUG] Parsing scenario payload: ${JSON.stringify(step.payload)}`);
            const parsedEvent = fromJson(WSEventSchema, step.payload);
            this.logger.info('MockTransport', `[DEBUG] Successfully parsed to Protobuf. Emitting...`);
            this.inbound$.next(parsedEvent);
          }
        } catch (err) {
           this.logger.error('MockTransport', `[DEBUG FATAL] Failed to parse scenario payload: ${JSON.stringify(step.payload)}`, err);
        }
      }

      if (step.awaitAction) {
        this.logger.info('MockTransport', `⏸️ Pausing scenario, waiting for user action: ${step.awaitAction}`);
        await new Promise(resolve => {
          this.pendingActionResolver = resolve;
          this.expectedAction = step.awaitAction!;
        });
        
        this.pendingActionResolver = null;
        this.expectedAction = null;
        this.logger.info('MockTransport', `▶️ Resuming scenario after receiving ${step.awaitAction}`);
      }
    }
  }

  private handleFakeTurn(promptText: string): void {
    const flowId = crypto.randomUUID();
    const taskId = 'task-' + flowId.substring(0, 5);
    const durableEventId = crypto.randomUUID();
    const messageNodeId = crypto.randomUUID();

    setTimeout(() => {
      const startAnchor = create(WSEventSchema, {
        payload: {
          case: 'streamedGraphEvent',
          value: {
            spaceId: 'project-alpha',
            chatId: 'chat-1',
            projectionId: 'main',
            event: {
              id: durableEventId,
              timestamp: new Date().toISOString(),
              tags: [],
              tombstoned: false,
              metadata: {},
              payload: {
                case: 'flowExecution',
                value: {
                  summary: 'Orchestrating agents...',
                  flowId: flowId,
                  taskId: taskId,
                  task: 'Synthesize Response',
                  status: 1, 
                  error: '',
                  agents: []
                }
              }
            }
          }
        }
      });
      this.inbound$.next(startAnchor);
    }, 100);

    setTimeout(() => {
      this.inbound$.next(fromJson(WSEventSchema, {
        telemetryEvent: {
          channel: 'manager',
          event: {
            id: crypto.randomUUID(),
            type: 'flow_start',
            timestamp: new Date().toISOString(),
            message: '',
            summary: 'Synthesizing response...',
            payloadJson: JSON.stringify({ flow_id: flowId, task_id: taskId, agent_count: 1 })
          }
        }
      }));
    }, 200);

    setTimeout(() => {
      this.inbound$.next(create(WSEventSchema, {
        payload: {
          case: 'streamedGraphEvent',
          value: {
            spaceId: 'project-alpha',
            chatId: 'chat-1',
            projectionId: 'main',
            event: {
              id: messageNodeId,
              timestamp: new Date().toISOString(),
              tags: [],
              tombstoned: false,
              metadata: {},
              payload: {
                case: 'message',
                value: { type: 2, text: '' }
              }
            }
          }
        }
      }));
    }, 300);

    setTimeout(() => {
      this.inbound$.next(fromJson(WSEventSchema, {
        chatStream: { nodeId: messageNodeId, text: `Mock response to: "${promptText}"` }
      }));
    }, 800);

    setTimeout(() => {
      this.inbound$.next(fromJson(WSEventSchema, {
        telemetryEvent: {
          channel: 'manager',
          event: {
            id: crypto.randomUUID(),
            type: 'flow_complete',
            timestamp: new Date().toISOString(),
            message: '',
            summary: '',
            payloadJson: JSON.stringify({ flow_id: flowId, agent_index: 1, passed: true })
          }
        }
      }));
    }, 1200);

    setTimeout(() => {
      const endAnchor = create(WSEventSchema, {
        payload: {
          case: 'streamedGraphEvent',
          value: {
            spaceId: 'project-alpha',
            chatId: 'chat-1',
            projectionId: 'main',
            event: {
              id: durableEventId, 
              timestamp: new Date().toISOString(),
              tags: [],
              tombstoned: false,
              metadata: {},
              payload: {
                case: 'flowExecution',
                value: {
                  summary: 'Response synthesized successfully.',
                  flowId: flowId,
                  taskId: taskId,
                  task: 'Synthesize Response',
                  status: 2, 
                  error: '',
                  agents: [{
                    agentId: 'mock-agent',
                    passed: true,
                    instruction: 'Write answer',
                    rawPayload: '{}',
                    verificationTrace: '[SYSTEM] OK',
                    stateDelta: ''
                  }]
                }
              }
            }
          }
        }
      });
      this.inbound$.next(endAnchor);
    }, 1300);
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