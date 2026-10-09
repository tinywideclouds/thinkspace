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
      
      case 'agentStream':
        return { type: 'agent_stream', agentId: protocolBufferEvent.payload.value.agentId, text: protocolBufferEvent.payload.value.text };
      
      case 'requestStrategy':
        return { type: 'request_strategy', active: protocolBufferEvent.payload.value.active };
      
      case 'requestReview':
        return { type: 'request_review', branch: protocolBufferEvent.payload.value.branch };

      case 'requestTransition':
        return { 
          type: 'request_transition', 
          targetDomain: protocolBufferEvent.payload.value.targetDomain, 
          reasoning: protocolBufferEvent.payload.value.reasoning 
        };
      
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

      case 'previewContext':
        return {
          type: 'preview_context',
          context: {
            spaceId: protocolBufferEvent.payload.value.spaceId,
            chatId: protocolBufferEvent.payload.value.chatId,
            managerSystemPrompt: protocolBufferEvent.payload.value.managerSystemPrompt,
            stickyDigests: protocolBufferEvent.payload.value.stickyDigests,
            recentHistory: protocolBufferEvent.payload.value.recentHistory,
            fileContexts: protocolBufferEvent.payload.value.fileContexts.map(fc => ({
              path: fc.path,
              level: fc.level,
              tokenCount: fc.tokenCount
            }))
          }
        };

      case 'telemetryEvent': {
        const te = protocolBufferEvent.payload.value;
        let parsedPayload = {};
        if (te.event?.payloadJson) {
          try {
            parsedPayload = JSON.parse(te.event.payloadJson);
          } catch (e) {
            console.warn(`[LlmFacade] Failed to parse telemetry payload_json:`, e);
          }
        }
        return {
          type: 'telemetry_event',
          channel: te.channel,
          id: te.event?.id || '',
          eventType: te.event?.type || '',
          timestamp: te.event?.timestamp || '',
          message: te.event?.message || '',
          summary: te.event?.summary || '',
          payload: parsedPayload
        };
      }
      
      default:
        console.warn(`[LlmFacade] Unhandled inbound protocol buffer case: ${protocolBufferEvent.payload.case}`);
        return null;
    }
  }

  static createSubmitPrompt(text: string, spaceId: string, chatId: string): WSEvent {
    return create(WSEventSchema, {
      payload: { case: 'submitPrompt', value: { text, spaceId, chatId } }
    });
  }

  static createApproveContext(approved: boolean, retainedFilePaths: string[] = [], retainedHistoryIndices: number[] = []): WSEvent {
    return create(WSEventSchema, {
      payload: { case: 'approveContext', value: { approved, retainedFilePaths, retainedHistoryIndices } }
    });
  }

  static createSelectStrategy(strategy: DomainDelegationStrategy): WSEvent {
    return create(WSEventSchema, {
      payload: { case: 'selectStrategy', value: { strategyId: strategy as unknown as DelegationStrategy } }
    });
  }

  static createReviewDecision(branch: string, accepted: boolean): WSEvent {
    return create(WSEventSchema, {
      payload: { case: 'reviewDecision', value: { branch, accepted } }
    });
  }

  static createTransitionDecision(approved: boolean): WSEvent {
    return create(WSEventSchema, {
      payload: { case: 'transitionDecision', value: { approved } }
    });
  }
}