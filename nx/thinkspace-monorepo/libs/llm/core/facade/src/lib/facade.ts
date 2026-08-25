import { create } from '@bufbuild/protobuf';
import { 
  WSEvent, 
  WSEventSchema,
  DelegationStrategy 
} from '@org/llm-core-protos';
import { DomainEvent, DomainDelegationStrategy } from './domain-models';

export class LlmFacade {
  /**
   * Translates an inbound Protobuf WSEvent into a clean DomainEvent.
   * Leverages the `oneof` case for strict type inference.
   */
  static toDomain(proto: WSEvent): DomainEvent | null {
    if (!proto.payload || proto.payload.case === undefined) {
      return null;
    }

    switch (proto.payload.case) {
      case 'chatStream':
        return { type: 'chat_stream', text: proto.payload.value.text };
      
      case 'logMessage':
        return { type: 'log_message', level: proto.payload.value.level, message: proto.payload.value.message };
      
      case 'delegationStart':
        return { 
          type: 'delegation_start', 
          agentCount: proto.payload.value.agentCount, 
          instructions: proto.payload.value.instructions 
        };
      
      case 'delegationComplete':
        return { type: 'delegation_complete', summary: proto.payload.value.summary };
      
      case 'agentStart':
        return { 
          type: 'agent_start', 
          agentId: proto.payload.value.agentId, 
          instructions: proto.payload.value.instructions 
        };
      
      case 'agentStream':
        return { type: 'agent_stream', agentId: proto.payload.value.agentId, text: proto.payload.value.text };
      
      case 'agentComplete':
        return { 
          type: 'agent_complete', 
          agentId: proto.payload.value.agentId, 
          branch: proto.payload.value.branch, 
          verified: proto.payload.value.verified 
        };
      
      case 'requestStrategy':
        return { type: 'request_strategy', active: proto.payload.value.active };
      
      case 'requestReview':
        return { type: 'request_review', branch: proto.payload.value.branch };
      
      case 'availableSpaces':
        return { 
          type: 'available_spaces', 
          spaces: proto.payload.value.spaces.map(s => ({ id: s.id, name: s.name })) 
        };
      
      default:
        console.warn(`[LlmFacade] Unhandled inbound proto case: ${proto.payload.case}`);
        return null;
    }
  }

  /**
   * Wraps a user prompt into the outbound Protobuf envelope safely using the schema.
   */
  static createSubmitPrompt(text: string, spaceId: string): WSEvent {
    return create(WSEventSchema, {
      payload: {
        case: 'submitPrompt',
        value: { text, spaceId }
      }
    });
  }

  /**
   * Wraps a user's strategy selection into the outbound Protobuf envelope safely using the schema.
   */
  static createSelectStrategy(strategy: DomainDelegationStrategy): WSEvent {
    const strategyId = strategy as unknown as DelegationStrategy;
    
    return create(WSEventSchema, {
      payload: {
        case: 'selectStrategy',
        value: { strategyId }
      }
    });
  }

  /**
   * Wraps a user's review decision into the outbound Protobuf envelope safely using the schema.
   */
  static createReviewDecision(branch: string, accepted: boolean): WSEvent {
    return create(WSEventSchema, {
      payload: {
        case: 'reviewDecision',
        value: { branch, accepted }
      }
    });
  }
}