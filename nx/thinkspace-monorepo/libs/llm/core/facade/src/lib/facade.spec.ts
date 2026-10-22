import { describe, it, expect } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { WSEventSchema, DelegationStrategy } from '@org/llm-core-protos';
import { LlmFacade } from './facade';
import { DomainDelegationStrategy } from './domain-models';

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

    it('should map an agent stream event correctly', () => {
      const protocolBufferEvent = create(WSEventSchema, {
        payload: {
          case: 'agentStream',
          value: { agentId: 1, text: 'thinking' }
        }
      });
      const domainEvent = LlmFacade.toDomain(protocolBufferEvent);
      expect(domainEvent).toEqual({ type: 'agent_stream', agentId: 1, text: 'thinking' });
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

    it('should map a sync history event with focus correctly', () => {
      const protocolBufferEvent = create(WSEventSchema, {
        payload: {
          case: 'syncHistory',
          value: {
            recentEvents: [],
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
        recentEvents: [],
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

    it('should map a preview context event with focus correctly', () => {
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
            targetDomain: 'golang',
            reasoning: 'Need to compile things'
          }
        }
      });

      const domainEvent = LlmFacade.toDomain(protocolBufferEvent);
      expect(domainEvent).toEqual({
        type: 'request_transition',
        targetDomain: 'golang',
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
        timestamp: '2026-09-02T15:00:00Z',
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

    it('should create a valid ApproveContext WSEvent with pruning arrays', () => {
      const protocolBufferEvent = LlmFacade.createApproveContext(true, ['src/main.go'], [0, 2]);
      
      expect(protocolBufferEvent.payload.case).toBe('approveContext');
      if (protocolBufferEvent.payload.case === 'approveContext') {
        expect(protocolBufferEvent.payload.value.approved).toBe(true);
        expect(protocolBufferEvent.payload.value.retainedFilePaths).toEqual(['src/main.go']);
        expect(protocolBufferEvent.payload.value.retainedHistoryIndices).toEqual([0, 2]);
      }
    });

    it('should create a valid SelectStrategy WSEvent', () => {
      const protocolBufferEvent = LlmFacade.createSelectStrategy(DomainDelegationStrategy.REFINE);
      
      expect(protocolBufferEvent.payload.case).toBe('selectStrategy');
      if (protocolBufferEvent.payload.case === 'selectStrategy') {
        expect(protocolBufferEvent.payload.value.strategyId).toBe(DelegationStrategy.REFINE);
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

    it('should create a valid TransitionDecision WSEvent', () => {
      const protocolBufferEvent = LlmFacade.createTransitionDecision(false);
      
      expect(protocolBufferEvent.payload.case).toBe('transitionDecision');
      if (protocolBufferEvent.payload.case === 'transitionDecision') {
        expect(protocolBufferEvent.payload.value.approved).toBe(false);
      }
    });
  });
});