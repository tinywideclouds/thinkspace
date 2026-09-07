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

    it('should map an available spaces event correctly', () => {
      const protocolBufferEvent = create(WSEventSchema, {
        payload: {
          case: 'availableSpaces',
          value: {
            spaces: [
              { id: 'golang', name: 'Go Developer' },
              { id: 'angular', name: 'Angular Expert' }
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

    it('should map a flow event correctly', () => {
      const protocolBufferEvent = create(WSEventSchema, {
        payload: {
          case: 'flowEvent',
          value: {
            flowId: 'flow-123',
            type: 'flow_status',
            timestamp: '2026-09-02T15:00:00Z',
            taskId: 'task-1',
            agentCount: 2,
            agentId: 'agent-1',
            agentIndex: 1,
            instruction: 'Do work',
            status: 'running_tests',
            attempt: 2,
            trace: 'compile error',
            candidateId: 'cand-1',
            passed: false
          }
        }
      });

      const domainEvent = LlmFacade.toDomain(protocolBufferEvent);
      expect(domainEvent).toEqual({
        type: 'flow_event',
        flowId: 'flow-123',
        eventType: 'flow_status',
        timestamp: '2026-09-02T15:00:00Z',
        taskId: 'task-1',
        agentCount: 2,
        agentId: 'agent-1',
        agentIndex: 1,
        instruction: 'Do work',
        status: 'running_tests',
        attempt: 2,
        trace: 'compile error',
        candidateId: 'cand-1',
        passed: false
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
  });
});