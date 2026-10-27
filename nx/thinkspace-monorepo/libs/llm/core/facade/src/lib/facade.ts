import { create } from '@bufbuild/protobuf';
import { 
  WSEvent, 
  WSEventSchema,
  DelegationStrategy 
} from '@org/llm-core-protos';
import { DomainEvent, DomainDelegationStrategy, DomainDigestMeta, DomainBranchTopology } from './domain-models';

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
          recentEvents: protocolBufferEvent.payload.value.recentEvents.map(event => ({
            id: event.id,
            timestamp: event.timestamp,
            type: event.type,
            content: event.content,
            metadata: event.metadata,
            edges: event.edges,
            branchId: event.branchId
          })),
          digests: Object.entries(protocolBufferEvent.payload.value.digests).reduce((accumulator, [key, digest]) => {
            accumulator[key] = { id: digest.id, summary: digest.summary, isSticky: digest.isSticky };
            return accumulator;
          }, {} as Record<string, DomainDigestMeta>),
          focus: {
            primaryFocus: protocolBufferEvent.payload.value.focus?.primaryFocus || [],
            relatedContext: protocolBufferEvent.payload.value.focus?.relatedContext || [],
            dormantLenses: protocolBufferEvent.payload.value.focus?.dormantLenses || []
          },
          nextCursor: protocolBufferEvent.payload.value.nextCursor,
          hasMore: protocolBufferEvent.payload.value.hasMore,
          activeBranch: protocolBufferEvent.payload.value.activeBranch,
          branches: Object.entries(protocolBufferEvent.payload.value.branches).reduce((accumulator, [key, topology]) => {
            accumulator[key] = { parentBranch: topology.parentBranch, forkPoint: topology.forkPoint };
            return accumulator;
          }, {} as Record<string, DomainBranchTopology>)
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
            fileContexts: protocolBufferEvent.payload.value.fileContexts.map(context => ({
              path: context.path,
              level: context.level,
              tokenCount: context.tokenCount
            })),
            focus: {
              primaryFocus: protocolBufferEvent.payload.value.focus?.primaryFocus || [],
              relatedContext: protocolBufferEvent.payload.value.focus?.relatedContext || [],
              dormantLenses: protocolBufferEvent.payload.value.focus?.dormantLenses || []
            }
          }
        };

      case 'telemetryEvent': {
        const telemetry = protocolBufferEvent.payload.value;
        let parsedPayload = {};
        if (telemetry.event?.payloadJson) {
          try {
            parsedPayload = JSON.parse(telemetry.event.payloadJson);
          } catch (error) {
            console.warn(`[LlmFacade] Failed to parse telemetry payload_json:`, error);
          }
        }
        return {
          type: 'telemetry_event',
          channel: telemetry.channel,
          id: telemetry.event?.id || '',
          eventType: telemetry.event?.type || '',
          timestamp: telemetry.event?.timestamp || '',
          message: telemetry.event?.message || '',
          summary: telemetry.event?.summary || '',
          payload: parsedPayload
        };
      }

      case 'streamedGraphEvent': {
        const event = protocolBufferEvent.payload.value.event;
        if (!event) return null;
        return {
          type: 'streamed_graph_event',
          event: {
            id: event.id,
            timestamp: event.timestamp,
            type: event.type,
            content: event.content,
            metadata: event.metadata,
            edges: event.edges,
            branchId: event.branchId
          }
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

  static createLoadHistory(spaceId: string, chatId: string, cursor: string, limit: number): WSEvent {
    return create(WSEventSchema, {
      payload: { case: 'loadHistory', value: { spaceId, chatId, cursor, limit } }
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

  static createForkChat(spaceId: string, chatId: string, targetEventId: string): WSEvent {
    return create(WSEventSchema, {
      payload: { case: 'forkChat', value: { spaceId, chatId, targetEventId } }
    });
  }

  static createInlineEdit(spaceId: string, chatId: string, targetEventId: string, newText: string): WSEvent {
    return create(WSEventSchema, {
      payload: { case: 'inlineEdit', value: { spaceId, chatId, targetEventId, newText } }
    });
  }

  static createSwitchBranch(spaceId: string, chatId: string, targetBranch: string): WSEvent {
    return create(WSEventSchema, {
      payload: { case: 'switchBranch', value: { spaceId, chatId, targetBranch } }
    });
  }
}