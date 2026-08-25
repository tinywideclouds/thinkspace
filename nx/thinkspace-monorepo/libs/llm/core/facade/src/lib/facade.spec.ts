import { describe, it, expect } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { WSEventSchema, DelegationStrategy } from '@org/llm-core-protos';
import { LlmFacade } from './facade';
import { DomainDelegationStrategy } from './domain-models';

describe('LlmFacade', () => {
  describe('toDomain (Inbound)', () => {
    it('should map a chat stream event correctly', () => {
      const proto = create(WSEventSchema, {
  payload: {
    case: 'chatStream',
    value: { text: 'Hello from model' }
  }
});
      const domain = LlmFacade.toDomain(proto);
      expect(domain).toEqual({ type: 'chat_stream', text: 'Hello from model' });
    });

    it('should map an available spaces event correctly', () => {
      const proto = create(WSEventSchema, {
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

      const domain = LlmFacade.toDomain(proto);
      expect(domain).toEqual({
        type: 'available_spaces',
        spaces: [
          { id: 'golang', name: 'Go Developer' },
          { id: 'angular', name: 'Angular Expert' }
        ]
      });
    });

    it('should return null for undefined payload or case', () => {
      const emptyProto = create(WSEventSchema);
      expect(LlmFacade.toDomain(emptyProto)).toBeNull();
    });
  });

  describe('Outbound Builders', () => {
    it('should create a valid SubmitPrompt WSEvent', () => {
      const proto = LlmFacade.createSubmitPrompt('Write a test', 'golang');
      
      expect(proto.payload.case).toBe('submitPrompt');
      if (proto.payload.case === 'submitPrompt') {
        expect(proto.payload.value.text).toBe('Write a test');
        expect(proto.payload.value.spaceId).toBe('golang');
      }
    });

    it('should create a valid SelectStrategy WSEvent', () => {
      const proto = LlmFacade.createSelectStrategy(DomainDelegationStrategy.REFINE);
      
      expect(proto.payload.case).toBe('selectStrategy');
      if (proto.payload.case === 'selectStrategy') {
        // Asserting it maps correctly to the underlying proto enum value (4)
        expect(proto.payload.value.strategyId).toBe(DelegationStrategy.REFINE);
      }
    });

    it('should create a valid ReviewDecision WSEvent', () => {
      const proto = LlmFacade.createReviewDecision('candidate/123', true);
      
      expect(proto.payload.case).toBe('reviewDecision');
      if (proto.payload.case === 'reviewDecision') {
        expect(proto.payload.value.branch).toBe('candidate/123');
        expect(proto.payload.value.accepted).toBe(true);
      }
    });
  });
});