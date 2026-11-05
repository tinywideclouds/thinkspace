import { create } from '@bufbuild/protobuf';
import { 
  WSEvent, 
  WSEventSchema,
  MessageType,
  FlowStatus,
  DelegationStrategy as ProtoDelegationStrategy,
  GraphEvent as ProtoGraphEvent
} from '@org/llm-core-protos';
import { 
  AppEvent, 
  DelegationStrategy, 
  DigestMeta, 
  BranchTopology, 
  GraphEvent 
} from './domain-models';

export class LlmFacade {
  
  private static mapFlowStatus(status: FlowStatus): 'UNSPECIFIED' | 'RUNNING' | 'COMPLETED' | 'FAILED' {
    switch (status) {
      case FlowStatus.RUNNING: return 'RUNNING';
      case FlowStatus.COMPLETED: return 'COMPLETED';
      case FlowStatus.FAILED: return 'FAILED';
      default: return 'UNSPECIFIED';
    }
  }

  private static mapGraphEvent(protoEvent: ProtoGraphEvent): GraphEvent | null {
    if (!protoEvent.payload || protoEvent.payload.case === undefined) {
      console.warn(`[LlmFacade] GraphEvent ${protoEvent.id} has no valid payload.`);
      return null;
    }

    const base = {
      id: protoEvent.id,
      timestamp: Temporal.Instant.from(protoEvent.timestamp),
      tags: protoEvent.tags || [],
      tombstoned: protoEvent.tombstoned,
      metadata: protoEvent.metadata || {},
      edges: protoEvent.edges || {},
      branchId: protoEvent.branchId || undefined
    };

    switch (protoEvent.payload.case) {
      case 'message': {
        const msgType = protoEvent.payload.value.type;
        let domainType: 'prompt' | 'model' | 'system' = 'prompt'; 
        
        if (msgType === MessageType.MODEL) {
          domainType = 'model';
        } else if (msgType === MessageType.SYSTEM) {
          domainType = 'system';
        }
        
        return {
          ...base,
          type: domainType,
          content: protoEvent.payload.value.text
        };
      }
      
      case 'fileDelta':
        return {
          ...base,
          type: 'file_delta',
          content: protoEvent.payload.value.summary,
          path: protoEvent.payload.value.path,
          previewSnippet: protoEvent.payload.value.previewSnippet,
          fileRef: protoEvent.payload.value.fileRef,
          parentId: protoEvent.payload.value.parentId || undefined
        };
      
      case 'flowExecution':
        return {
          ...base,
          type: 'flow_execution',
          content: protoEvent.payload.value.summary,
          flowId: protoEvent.payload.value.flowId,
          taskId: protoEvent.payload.value.taskId,
          task: protoEvent.payload.value.task,
          status: this.mapFlowStatus(protoEvent.payload.value.status),
          error: protoEvent.payload.value.error,
          agents: protoEvent.payload.value.agents.map(a => ({
            agentId: a.agentId,
            passed: a.passed,
            instruction: a.instruction,
            rawPayload: a.rawPayload,
            verificationTrace: a.verificationTrace,
            stateDelta: a.stateDelta
          }))
        };

      default:
        console.warn(`[LlmFacade] Unknown GraphEvent payload: ${protoEvent.payload}`);
        return null;
    }
  }

  static toDomain(protocolBufferEvent: WSEvent): AppEvent | null {
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
          targetThinkState: protocolBufferEvent.payload.value.targetThinkState, 
          reasoning: protocolBufferEvent.payload.value.reasoning 
        };
      
      case 'availableSpaces':
        return { type: 'available_spaces', spaces: protocolBufferEvent.payload.value.spaces.map(space => ({ id: space.id, name: space.name })) };
      
      case 'syncHistory': {
        const validEvents = protocolBufferEvent.payload.value.recentEvents
          .map(e => this.mapGraphEvent(e))
          .filter((e): e is GraphEvent => e !== null);

        return {
          type: 'sync_history',
          recentEvents: validEvents,
          digests: Object.entries(protocolBufferEvent.payload.value.digests).reduce((accumulator, [key, digest]) => {
            accumulator[key] = { id: digest.id, summary: digest.summary, isSticky: digest.isSticky };
            return accumulator;
          }, {} as Record<string, DigestMeta>),
          focus: {
            primaryFocus: protocolBufferEvent.payload.value.focus?.primaryFocus || [],
            relatedContext: protocolBufferEvent.payload.value.focus?.relatedContext || [],
            dormantLenses: protocolBufferEvent.payload.value.focus?.dormantLenses || []
          },
          nextCursor: protocolBufferEvent.payload.value.nextCursor,
          hasMore: protocolBufferEvent.payload.value.hasMore,
          activeBranch: protocolBufferEvent.payload.value.activeBranch || 'main',
          branches: Object.entries(protocolBufferEvent.payload.value.branches).reduce((accumulator, [key, topology]) => {
            accumulator[key] = { parentBranch: topology.parentBranch, forkPoint: topology.forkPoint };
            return accumulator;
          }, {} as Record<string, BranchTopology>)
        };
      }

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
          timestamp: telemetry.event?.timestamp ? Temporal.Instant.from(telemetry.event.timestamp) : Temporal.Now.instant(),
          message: telemetry.event?.message || '',
          summary: telemetry.event?.summary || '',
          payload: parsedPayload
        };
      }

      case 'streamedGraphEvent': {
        const protoGraphEvent = protocolBufferEvent.payload.value.event;
        if (!protoGraphEvent) return null;
        
        const mappedDomainEvent = this.mapGraphEvent(protoGraphEvent);
        if (!mappedDomainEvent) return null;

        return {
          type: 'streamed_graph_event',
          event: mappedDomainEvent
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

  static createSelectStrategy(strategy: DelegationStrategy): WSEvent {
    return create(WSEventSchema, {
      payload: { case: 'selectStrategy', value: { strategyId: strategy as unknown as ProtoDelegationStrategy } }
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