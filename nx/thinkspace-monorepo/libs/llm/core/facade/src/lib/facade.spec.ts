import { describe, it, expect } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { WSEventSchema, DelegationStrategy as ProtoDelegationStrategy, MessageType, FlowStatus } from '@org/llm-core-protos';
import { LlmFacade } from './facade';
import { DelegationStrategy } from './domain-models';

describe('LlmFacade', () => {
  describe('toDomain (Inbound)', () => {
    it('should map a chat stream event correctly', () => {
      const protocolBufferEvent = create(WSEventSchema, {
        payload: {
          case: 'chatStream',
          value: { text: 'Hello from model' }
        }
      });
      const domainEvent = LlmFacade.toDomain(protocolBufferEvent);
      expect(domainEvent).toEqual({ type: 'chat_stream', text: 'Hello from model' });
    });

    it('should map a streamed graph event (File Delta) correctly using the strict payload', () => {
      const protocolBufferEvent = create(WSEventSchema, {
        payload: {
          case: 'streamedGraphEvent',
          value: {
            spaceId: 'golang',
            chatId: 'chat-1',
            projectionId: 'main',
            event: {
              id: 'event-1',
              timestamp: '2026-10-23T12:00:00Z',
              edges: { amended_by: 'event-2' },
              tags: ['ui'],
              metadata: { custom_info: 'test' },
              payload: {
                case: 'fileDelta',
                value: {
                  summary: 'test content',
                  path: 'src/main.ts',
                  previewSnippet: 'patch_data',
                  fileRef: 'ref-123',
                  parentId: 'model-1'
                }
              }
            }
          }
        }
      });
      
      const domainEvent = LlmFacade.toDomain(protocolBufferEvent);
      expect(domainEvent).toEqual({
        type: 'streamed_graph_event',
        coordinate: { spaceId: 'golang', chatId: 'chat-1', projectionId: 'main' },
        event: {
          id: 'event-1',
          type: 'file_delta',
          timestamp: expect.anything(), 
          tags: ['ui'],
          content: 'test content',
          path: 'src/main.ts',
          previewSnippet: 'patch_data',
          fileRef: 'ref-123',
          parentId: 'model-1',
          tombstoned: false,
          metadata: { custom_info: 'test' },
          edges: { amended_by: 'event-2' }
        }
      });
    });

    it('should map a streamed graph event (Flow Execution) correctly', () => {
      const protocolBufferEvent = create(WSEventSchema, {
        payload: {
          case: 'streamedGraphEvent',
          value: {
            spaceId: 'golang',
            chatId: 'chat-1',
            projectionId: 'main',
            event: {
              id: 'event-3',
              timestamp: '2026-10-23T12:00:00Z',
              payload: {
                case: 'flowExecution',
                value: {
                  summary: 'Flow running',
                  flowId: 'flow-123',
                  taskId: 'task-123',
                  task: 'Build the thing',
                  status: FlowStatus.RUNNING,
                  error: '',
                  agents: [
                    {
                      agentId: 'agent-1',
                      passed: false,
                      instruction: 'do work',
                      rawPayload: '{}',
                      verificationTrace: '',
                      stateDelta: ''
                    }
                  ]
                }
              }
            }
          }
        }
      });
      
      const domainEvent = LlmFacade.toDomain(protocolBufferEvent);
      expect(domainEvent).toEqual({
        type: 'streamed_graph_event',
        coordinate: { spaceId: 'golang', chatId: 'chat-1', projectionId: 'main' },
        event: {
          id: 'event-3',
          type: 'flow_execution',
          timestamp: expect.anything(),
          tags: [],
          content: 'Flow running',
          flowId: 'flow-123',
          taskId: 'task-123',
          task: 'Build the thing',
          status: 'RUNNING',
          error: '',
          agents: [
            {
              agentId: 'agent-1',
              passed: false,
              instruction: 'do work',
              rawPayload: '{}',
              verificationTrace: '',
              stateDelta: ''
            }
          ],
          tombstoned: false,
          metadata: {},
          edges: {},
        }
      });
    });

    it('should map a streamed graph event (Message) correctly', () => {
      const protocolBufferEvent = create(WSEventSchema, {
        payload: {
          case: 'streamedGraphEvent',
          value: {
            spaceId: 'golang',
            chatId: 'chat-1',
            projectionId: 'main',
            event: {
              id: 'event-2',
              timestamp: '2026-10-23T12:05:00Z',
              payload: {
                case: 'message',
                value: {
                  type: MessageType.MODEL,
                  text: 'Here is the response'
                }
              }
            }
          }
        }
      });
      
      const domainEvent = LlmFacade.toDomain(protocolBufferEvent);
      expect(domainEvent).toEqual({
        type: 'streamed_graph_event',
        coordinate: { spaceId: 'golang', chatId: 'chat-1', projectionId: 'main' },
        event: {
          id: 'event-2',
          type: 'model',
          timestamp: expect.anything(), 
          tags: [],
          content: 'Here is the response',
          tombstoned: false,
          metadata: {},
          edges: {},
        }
      });
    });

    it('should map an available spaces event correctly', () => {
      const protocolBufferEvent = create(WSEventSchema, {
        payload: {
          case: 'availableSpaces',
          value: {
            spaces: [
              { id: 'golang', name: 'Go Developer', isConfigured: true },
              { id: 'angular', name: 'Angular Expert', isConfigured: false }
            ]
          }
        }
      });

      const domainEvent = LlmFacade.toDomain(protocolBufferEvent);
      expect(domainEvent).toEqual({
        type: 'available_spaces',
        spaces: [
          { id: 'golang', name: 'Go Developer' },
          { id: 'angular', name: 'Angular Expert' }
        ]
      });
    });

    it('should map a sync history event correctly and map the envelope coordinate', () => {
      const protocolBufferEvent = create(WSEventSchema, {
        payload: {
          case: 'syncHistory',
          value: {
            spaceId: 'golang',
            chatId: 'chat-2',
            projectionId: 'branch-x',
            recentEvents: [
              {
                id: 'event-broken',
                timestamp: '2026-10-23T12:05:00Z',
              },
              {
                id: 'event-valid',
                timestamp: '2026-10-23T12:06:00Z',
                payload: {
                  case: 'message',
                  value: {
                    type: MessageType.PROMPT,
                    text: 'Valid prompt'
                  }
                }
              }
            ],
            digests: {},
            focus: {
              primaryFocus: ['architecture'],
              relatedContext: ['auth'],
              dormantLenses: ['database']
            },
            nextCursor: 'cursor-123',
            hasMore: true
          }
        }
      });

      const domainEvent = LlmFacade.toDomain(protocolBufferEvent);
      expect(domainEvent).toEqual({
        type: 'sync_history',
        coordinate: { spaceId: 'golang', chatId: 'chat-2', projectionId: 'branch-x' },
        recentEvents: [
          {
            id: 'event-valid',
            type: 'prompt',
            timestamp: expect.anything(), 
            tags: [],
            content: 'Valid prompt',
            tombstoned: false,
            metadata: {},
            edges: {},
          }
        ],
        digests: {},
        focus: {
          primaryFocus: ['architecture'],
          relatedContext: ['auth'],
          dormantLenses: ['database']
        },
        nextCursor: 'cursor-123',
        hasMore: true
      });
    });

    it('should map a sync subgraph event correctly and map the envelope coordinate', () => {
      const protocolBufferEvent = create(WSEventSchema, {
        payload: {
          case: 'syncSubGraph',
          value: {
            spaceId: 'golang',
            chatId: 'chat-3',
            projectionId: 'flow-y',
            events: []
          }
        }
      });

      const domainEvent = LlmFacade.toDomain(protocolBufferEvent);
      expect(domainEvent).toEqual({
        type: 'sync_subgraph',
        coordinate: { spaceId: 'golang', chatId: 'chat-3', projectionId: 'flow-y' },
        events: []
      });
    });

    it('should map a preview context event correctly', () => {
      const protocolBufferEvent = create(WSEventSchema, {
        payload: {
          case: 'previewContext',
          value: {
            spaceId: 'golang',
            chatId: 'chat-1',
            managerSystemPrompt: 'Test rules',
            stickyDigests: ['Sticky memory'],
            fileContexts: [
              { path: 'src/main.go', level: 'structural', tokenCount: 100 }
            ],
            focus: {
              primaryFocus: ['golang'],
              relatedContext: [],
              dormantLenses: []
            }
          }
        }
      });

      const domainEvent = LlmFacade.toDomain(protocolBufferEvent);
      expect(domainEvent).toEqual({
        type: 'preview_context',
        context: {
          spaceId: 'golang',
          chatId: 'chat-1',
          managerSystemPrompt: 'Test rules',
          stickyDigests: ['Sticky memory'],
          recentHistory: [],
          fileContexts: [
            { path: 'src/main.go', level: 'structural', tokenCount: 100 }
          ],
          focus: {
            primaryFocus: ['golang'],
            relatedContext: [],
            dormantLenses: []
          }
        }
      });
    });

    it('should map a request transition event correctly', () => {
      const protocolBufferEvent = create(WSEventSchema, {
        payload: {
          case: 'requestTransition',
          value: {
            targetThinkState: 'golang',
            reasoning: 'Need to compile things'
          }
        }
      });

      const domainEvent = LlmFacade.toDomain(protocolBufferEvent);
      expect(domainEvent).toEqual({
        type: 'request_transition',
        targetThinkState: 'golang',
        reasoning: 'Need to compile things'
      });
    });

    it('should map a telemetry event correctly and parse the JSON payload', () => {
      const protocolBufferEvent = create(WSEventSchema, {
        payload: {
          case: 'telemetryEvent',
          value: {
            channel: 'agent-1',
            event: {
              id: 'uuid-123',
              type: 'flow_status',
              timestamp: '2026-09-02T15:00:00Z',
              message: 'Compiling Go AST...',
              summary: 'Testing AST',
              payloadJson: '{"attempt": 2, "passed": true}'
            }
          }
        }
      });

      const domainEvent = LlmFacade.toDomain(protocolBufferEvent);
      expect(domainEvent).toEqual({
        type: 'telemetry_event',
        channel: 'agent-1',
        id: 'uuid-123',
        eventType: 'flow_status',
        timestamp: expect.anything(), 
        message: 'Compiling Go AST...',
        summary: 'Testing AST',
        payload: {
          attempt: 2,
          passed: true
        }
      });
    });

    it('should return null for undefined payload or case', () => {
      const emptyProtocolBufferEvent = create(WSEventSchema);
      expect(LlmFacade.toDomain(emptyProtocolBufferEvent)).toBeNull();
    });
  });

  describe('Outbound Builders', () => {
    it('should create a valid SubmitPrompt WSEvent', () => {
      const protocolBufferEvent = LlmFacade.createSubmitPrompt('Write a test', 'golang', 'test-chat');
      
      expect(protocolBufferEvent.payload.case).toBe('submitPrompt');
      if (protocolBufferEvent.payload.case === 'submitPrompt') {
        expect(protocolBufferEvent.payload.value.text).toBe('Write a test');
        expect(protocolBufferEvent.payload.value.spaceId).toBe('golang');
        expect(protocolBufferEvent.payload.value.chatId).toBe('test-chat');
      }
    });

    it('should create a valid LoadHistory WSEvent', () => {
      const protocolBufferEvent = LlmFacade.createLoadHistory('golang', 'chat-1', 'cursor-123', 20);
      
      expect(protocolBufferEvent.payload.case).toBe('loadHistory');
      if (protocolBufferEvent.payload.case === 'loadHistory') {
        expect(protocolBufferEvent.payload.value.spaceId).toBe('golang');
        expect(protocolBufferEvent.payload.value.chatId).toBe('chat-1');
        expect(protocolBufferEvent.payload.value.cursor).toBe('cursor-123');
        expect(protocolBufferEvent.payload.value.limit).toBe(20);
      }
    });

    it('should create a valid ApproveContext WSEvent', () => {
      const protocolBufferEvent = LlmFacade.createApproveContext(true, ['src/main.go'], [0, 2]);
      
      expect(protocolBufferEvent.payload.case).toBe('approveContext');
      if (protocolBufferEvent.payload.case === 'approveContext') {
        expect(protocolBufferEvent.payload.value.approved).toBe(true);
        expect(protocolBufferEvent.payload.value.retainedFilePaths).toEqual(['src/main.go']);
        expect(protocolBufferEvent.payload.value.retainedHistoryIndices).toEqual([0, 2]);
      }
    });

    it('should create a valid SelectStrategy WSEvent', () => {
      const protocolBufferEvent = LlmFacade.createSelectStrategy(DelegationStrategy.REFINE);
      
      expect(protocolBufferEvent.payload.case).toBe('selectStrategy');
      if (protocolBufferEvent.payload.case === 'selectStrategy') {
        expect(protocolBufferEvent.payload.value.strategyId).toBe(ProtoDelegationStrategy.REFINE);
      }
    });

    it('should create a valid ReviewDecision WSEvent', () => {
      const protocolBufferEvent = LlmFacade.createReviewDecision('candidate/123', true);
      
      expect(protocolBufferEvent.payload.case).toBe('reviewDecision');
      if (protocolBufferEvent.payload.case === 'reviewDecision') {
        expect(protocolBufferEvent.payload.value.branch).toBe('candidate/123');
        expect(protocolBufferEvent.payload.value.accepted).toBe(true);
      }
    });

    it('should create a valid LoadSubGraph WSEvent', () => {
      const protocolBufferEvent = LlmFacade.createLoadSubGraph('golang', 'chat-1', 'flow-123');
      
      expect(protocolBufferEvent.payload.case).toBe('loadSubGraph');
      if (protocolBufferEvent.payload.case === 'loadSubGraph') {
        expect(protocolBufferEvent.payload.value.spaceId).toBe('golang');
        expect(protocolBufferEvent.payload.value.chatId).toBe('chat-1');
        expect(protocolBufferEvent.payload.value.graphId).toBe('flow-123');
      }
    });

    it('should create a valid TransitionDecision WSEvent', () => {
      const protocolBufferEvent = LlmFacade.createTransitionDecision(false);
      
      expect(protocolBufferEvent.payload.case).toBe('transitionDecision');
      if (protocolBufferEvent.payload.case === 'transitionDecision') {
        expect(protocolBufferEvent.payload.value.approved).toBe(false);
      }
    });

    it('should create a valid ForkChat WSEvent', () => {
      const protocolBufferEvent = LlmFacade.createForkChat('golang', 'chat-1', 'event-456');
      
      expect(protocolBufferEvent.payload.case).toBe('forkChat');
      if (protocolBufferEvent.payload.case === 'forkChat') {
        expect(protocolBufferEvent.payload.value.spaceId).toBe('golang');
        expect(protocolBufferEvent.payload.value.chatId).toBe('chat-1');
        expect(protocolBufferEvent.payload.value.targetEventId).toBe('event-456');
      }
    });

    it('should create a valid InlineEdit WSEvent', () => {
      const protocolBufferEvent = LlmFacade.createInlineEdit('golang', 'chat-1', 'event-456', 'New text');
      
      expect(protocolBufferEvent.payload.case).toBe('inlineEdit');
      if (protocolBufferEvent.payload.case === 'inlineEdit') {
        expect(protocolBufferEvent.payload.value.spaceId).toBe('golang');
        expect(protocolBufferEvent.payload.value.chatId).toBe('chat-1');
        expect(protocolBufferEvent.payload.value.targetEventId).toBe('event-456');
        expect(protocolBufferEvent.payload.value.newText).toBe('New text');
      }
    });

    it('should create a valid SwitchBranch WSEvent', () => {
      const protocolBufferEvent = LlmFacade.createSwitchBranch('golang', 'chat-1', 'feature-branch');
      
      expect(protocolBufferEvent.payload.case).toBe('switchBranch');
      if (protocolBufferEvent.payload.case === 'switchBranch') {
        expect(protocolBufferEvent.payload.value.spaceId).toBe('golang');
        expect(protocolBufferEvent.payload.value.chatId).toBe('chat-1');
        expect(protocolBufferEvent.payload.value.targetBranch).toBe('feature-branch');
      }
    });
  });
});