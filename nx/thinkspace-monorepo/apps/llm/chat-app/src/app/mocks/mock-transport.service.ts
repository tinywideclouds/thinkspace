import { Injectable, inject } from '@angular/core';
import { Observable, Subject } from 'rxjs';
import { WSEvent, WSEventSchema, GraphEventSchema } from '@org/llm-core-protos';
import { create, fromJson, JsonValue } from '@bufbuild/protobuf';
import { UiSettingsService } from '@org/llm-state-chat';
import { LoggerService } from '@org/core-logger';
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
  private logger = inject(LoggerService);
  private inbound$ = new Subject<WSEvent>();
  private allEvents = [...mockGraph.events];
  private uiSettings = inject(UiSettingsService);
  private currentLatencyMs = 500;

  constructor() {
    this.validateMockGraph();
  }

  private validateMockGraph(): void {
    let hasErrors = false;
    this.logger.info('MockTransport', 'Validating mock-graph.json structure...');

    this.allEvents.forEach((event: any, index: number) => {
      if (event.type || event.payload?.case) {
        this.logger.error('MockTransport', `[Invalid History Event] Index ${index} (ID: ${event.id}): Uses legacy format. Expected standard Protobuf JSON.`);
        hasErrors = true;
      }
    });

    const scenarios = mockGraph.scenarios as Record<string, MockScenarioStep[]>;
    for (const [scenarioName, steps] of Object.entries(scenarios)) {
      steps.forEach((step, index) => {
        if (step.payload) {
          if (step.payload.case || (step.payload.payload && step.payload.payload.case)) {
            this.logger.error('MockTransport', `[Invalid Scenario Step] Scenario "${scenarioName}" Step ${index}: Uses legacy "case/value" format.`);
            hasErrors = true;
          }
          if (step.payload.streamedGraphEvent?.event?.type) {
            this.logger.error('MockTransport', `[Invalid Scenario Step] Scenario "${scenarioName}" Step ${index}: Nested GraphEvent uses legacy format.`);
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

  private handleLoadHistory(payload: { cursor?: string; limit: number }): void {
    setTimeout(() => {
      let endIndex = this.allEvents.length;
      if (payload.cursor) {
        const cursorIdx = this.allEvents.findIndex((e: any) => e.id === payload.cursor);
        if (cursorIdx !== -1) endIndex = cursorIdx;
      }

      const startIndex = Math.max(0, endIndex - payload.limit);
      const page = this.allEvents.slice(startIndex, endIndex);

      const hasMore = startIndex > 0;
      const nextCursor = page.length > 0 ? (page[0] as any).id : (payload.cursor || '');

      try {
        const mappedEvents = page.map((e: any) => fromJson(GraphEventSchema, e as JsonValue));

        this.inbound$.next(create(WSEventSchema, {
          payload: {
            case: 'syncHistory',
            value: {
              recentEvents: mappedEvents,
              digests: payload.cursor ? {} : mockGraph.digests as Record<string, any>,
              focus: payload.cursor ? undefined : mockGraph.focus as any,
              nextCursor,
              hasMore
            }
          }
        }));
      } catch (err) {
        this.logger.error('MockTransport', 'FATAL: Failed to construct syncHistory payload!', err);
      }
    }, this.currentLatencyMs);
  }

  private playScenario(name: string): void {
    const scenarioData = (mockGraph.scenarios as Record<string, MockScenarioStep[]>)[name];
    if (!scenarioData || !Array.isArray(scenarioData)) {
      this.logger.warn('MockTransport', `Scenario "${name}" not found in mock-graph.scenarios`);
      return;
    }

    this.logger.info('MockTransport', `🎬 Playing scenario: ${name}`);
    for (const step of scenarioData) {
      setTimeout(() => {
        if (step.latencyMs !== undefined) {
          this.currentLatencyMs = step.latencyMs;
        }

        if (step.uiSettings?.pagination !== undefined) {
          this.uiSettings.pagination.set(step.uiSettings.pagination);
        }

        if (!step.payload) return;

        try {
          if (step.payload.syncHistory) {
            this.allEvents = step.payload.syncHistory.recentEvents;
            this.handleLoadHistory({ limit: this.uiSettings.pagination().size, cursor: '' });
          } else {
            this.inbound$.next(fromJson(WSEventSchema, step.payload as JsonValue));
          }
        } catch (err) {
           this.logger.error('MockTransport', `FATAL: Failed to parse scenario payload!`, err);
        }
      }, step.offsetMs);
    }
  }

  private handleFakeTurn(promptText: string): void {
    const flowId = crypto.randomUUID();
    const taskId = 'task-' + flowId.substring(0, 5);
    const durableEventId = crypto.randomUUID();

    // 1. Drop the durable anchor into the timeline first
    setTimeout(() => {
      const startAnchor = create(WSEventSchema, {
        payload: {
          case: 'streamedGraphEvent',
          value: {
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
                  status: 1, // FlowStatus.RUNNING
                  error: '',
                  agents: []
                }
              }
            }
          }
        }
      });
      console.log('[DEBUG MockTransport] Emitting Start Anchor:', startAnchor);
      this.inbound$.next(startAnchor);
    }, 100);

    // 2. Fire the ephemeral telemetry for the live overlay
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
      } as JsonValue));
    }, 200);

    // 3. Emit standard model response
    setTimeout(() => {
      this.inbound$.next(fromJson(WSEventSchema, {
        chatStream: { text: `Mock response to: "${promptText}"` }
      } as JsonValue));
    }, 800);

    // 4. Complete the live telemetry
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
      } as JsonValue));
    }, 1200);

    // 5. Update the durable anchor to COMPLETED
    setTimeout(() => {
      const endAnchor = create(WSEventSchema, {
        payload: {
          case: 'streamedGraphEvent',
          value: {
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
                  status: 2, // FlowStatus.COMPLETED
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
      console.log('[DEBUG MockTransport] Emitting End Anchor:', endAnchor);
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