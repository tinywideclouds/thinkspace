import { create } from '@bufbuild/protobuf';
import { 
  WSEvent, 
  WSEventSchema,
  DelegationStrategy 
} from '@org/llm-core-protos';
import { DomainEvent, DomainDelegationStrategy, DomainDigestMeta } from './domain-models';

export class LlmFacade {
  static toDomain(protocolBufferEvent: WSEvent): DomainEvent | null {
    if (!protocolBufferEvent.payload || protocolBufferEvent.payload.case === undefined) {
      return null;
    }

    switch (protocolBufferEvent.payload.case) {
      case 'chatStream':
        return { type: 'chat_stream', text: protocolBufferEvent.payload.value.text };
      
      case 'logMessage':
        return { type: 'log_message', level: protocolBufferEvent.payload.value.level, message: protocolBufferEvent.payload.value.message };
      
      case 'agentStream':
        return { type: 'agent_stream', agentId: protocolBufferEvent.payload.value.agentId, text: protocolBufferEvent.payload.value.text };
      
      case 'requestStrategy':
        return { type: 'request_strategy', active: protocolBufferEvent.payload.value.active };
      
      case 'requestReview':
        return { type: 'request_review', branch: protocolBufferEvent.payload.value.branch };
      
      case 'availableSpaces':
        return { type: 'available_spaces', spaces: protocolBufferEvent.payload.value.spaces.map(space => ({ id: space.id, name: space.name })) };
      
      case 'syncHistory':
        return {
          type: 'sync_history',
          recentEvents: protocolBufferEvent.payload.value.recentEvents.map(e => ({
            id: e.id,
            timestamp: e.timestamp,
            type: e.type,
            content: e.content,
            metadata: e.metadata
          })),
          digests: Object.entries(protocolBufferEvent.payload.value.digests).reduce((acc, [key, val]) => {
            acc[key] = { id: val.id, summary: val.summary, isSticky: val.isSticky };
            return acc;
          }, {} as Record<string, DomainDigestMeta>)
        };

      case 'flowEvent':
        return {
          type: 'flow_event',
          flowId: protocolBufferEvent.payload.value.flowId,
          eventType: protocolBufferEvent.payload.value.type,
          timestamp: protocolBufferEvent.payload.value.timestamp,
          taskId: protocolBufferEvent.payload.value.taskId,
          agentCount: protocolBufferEvent.payload.value.agentCount,
          agentId: protocolBufferEvent.payload.value.agentId,
          agentIndex: protocolBufferEvent.payload.value.agentIndex,
          instruction: protocolBufferEvent.payload.value.instruction,
          status: protocolBufferEvent.payload.value.status,
          attempt: protocolBufferEvent.payload.value.attempt,
          trace: protocolBufferEvent.payload.value.trace,
          candidateId: protocolBufferEvent.payload.value.candidateId,
          passed: protocolBufferEvent.payload.value.passed
        };
      
      default:
        console.warn(`[LlmFacade] Unhandled inbound protocol buffer case: ${protocolBufferEvent.payload.case}`);
        return null;
    }
  }

  static createSubmitPrompt(text: string, spaceId: string, chatId: string): WSEvent {
    return create(WSEventSchema, {
      payload: {
        case: 'submitPrompt',
        value: { text, spaceId, chatId }
      }
    });
  }

  static createSelectStrategy(strategy: DomainDelegationStrategy): WSEvent {
    const strategyId = strategy as unknown as DelegationStrategy;
    return create(WSEventSchema, {
      payload: {
        case: 'selectStrategy',
        value: { strategyId }
      }
    });
  }

  static createReviewDecision(branch: string, accepted: boolean): WSEvent {
    return create(WSEventSchema, {
      payload: {
        case: 'reviewDecision',
        value: { branch, accepted }
      }
    });
  }
}