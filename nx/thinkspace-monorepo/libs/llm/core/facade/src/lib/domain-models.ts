export interface SpaceInfo {
  id: string;
  name: string;
}

export type GraphEventType =
  | 'prompt'
  | 'model'
  | 'system'
  | 'file_delta'
  | 'flow_execution'
  | 'candidate_resolution';

export interface BaseGraphEvent {
  id: string;
  timestamp: Temporal.Instant;
  tags: string[];
  tombstoned: boolean;
  metadata: Record<string, string>;
  edges: Record<string, string>;
}

export interface MessageGraphEvent extends BaseGraphEvent {
  type: 'prompt' | 'model' | 'system';
  content: string;
}

export interface FileDeltaGraphEvent extends BaseGraphEvent {
  type: 'file_delta';
  content: string;
  path: string;
  previewSnippet: string;
  fileRef: string;
  parentId?: string;
}

export interface CandidateResolutionGraphEvent extends BaseGraphEvent {
  type: 'candidate_resolution';
  content: string;
  candidateId: string;
  accepted: boolean;
  reason: string;
}

export interface AgentRecord {
  agentId: string;
  passed: boolean;
  instruction: string;
  rawPayload: string;
  verificationTrace: string;
  stateDelta: string;
}

export interface FlowExecutionGraphEvent extends BaseGraphEvent {
  type: 'flow_execution';
  content: string;
  flowId: string;
  taskId: string;
  task: string;
  status: 'UNSPECIFIED' | 'RUNNING' | 'COMPLETED' | 'FAILED';
  error: string;
  agents: AgentRecord[];
}

export type GraphEvent =
  | MessageGraphEvent
  | FileDeltaGraphEvent
  | CandidateResolutionGraphEvent
  | FlowExecutionGraphEvent;

export interface ProjectionCoordinate {
  spaceId: string;
  chatId: string;
  projectionId: string;
}

export interface DigestMeta {
  id: string;
  summary: string;
  isSticky: boolean;
}

export interface FileContextMetaData {
  path: string;
  level: string;
  tokenCount: number;
}

export interface AttentionFocus {
  primaryFocus: string[];
  relatedContext: string[];
  dormantLenses: string[];
}

export enum ContextStage {
  UNSPECIFIED = 0,
  MANAGER = 1,
  WORKER = 2,
}

export interface PreviewContext {
  spaceId: string;
  chatId: string;
  previewId?: string;
  stage: ContextStage;
  managerSystemPrompt: string;
  longTermMemory: string[];
  recentHistory: string[];
  fileContexts: FileContextMetaData[];
  focus: AttentionFocus;
}

export interface FlowTelemetryPayload {
  type: 'flow';
  flowId: string;
  taskId: string;
  agentCount: number;
}

export interface AgentTelemetryPayload {
  type: 'agent';
  flowId: string;
  agentId: string;
  agentIndex: number;
  instruction: string;
  attempt: number;
  trace: string;
  candidateId: string;
  passed: boolean;
}

export interface TransitionTelemetryPayload {
  type: 'transition';
  targetThinkState: string;
}

export type TelemetryPayload =
  | FlowTelemetryPayload
  | AgentTelemetryPayload
  | TransitionTelemetryPayload;

export interface TelemetryEvent {
  type: 'telemetry_event';
  channel: string;
  id: string;
  eventType: string;
  timestamp: Temporal.Instant;
  message: string;
  summary: string;
  payload?: TelemetryPayload;
}

export type AppEvent =
  | { type: 'chat_stream'; nodeId: string; text: string }
  | { type: 'request_strategy'; active: boolean }
  | { type: 'request_review'; branch: string }
  | { type: 'request_transition'; targetThinkState: string; reasoning: string }
  | { type: 'available_spaces'; spaces: SpaceInfo[] }
  | {
      type: 'sync_history';
      recentEvents: GraphEvent[];
      digests: Record<string, DigestMeta>;
      focus: AttentionFocus;
      nextCursor: string;
      hasMore: boolean;
      coordinate: ProjectionCoordinate;
    }
  | { type: 'sync_subgraph'; events: GraphEvent[]; coordinate: ProjectionCoordinate }
  | { type: 'preview_context'; context: PreviewContext }
  | { type: 'streamed_graph_event'; event: GraphEvent; coordinate: ProjectionCoordinate }
  | TelemetryEvent;

export enum DelegationStrategy {
  SKIP = 1,
  MANUAL = 2,
  REVIEW = 3,
  REFINE = 4,
}