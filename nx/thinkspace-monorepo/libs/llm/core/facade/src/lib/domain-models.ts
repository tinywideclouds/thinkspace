export interface SpaceInfo {
  id: string;
  name: string;
}

export type GraphEventType = 'prompt' | 'model' | 'system' | 'file_delta' | 'flow_execution';

export interface BaseGraphEvent {
  id: string;
  timestamp: Temporal.Instant; // Mapped to native Temporal API
  tags: string[];
  tombstoned: boolean;
  metadata: Record<string, string>;
  edges: Record<string, string>;
  branchId?: string;
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
  | FlowExecutionGraphEvent;

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

export interface PreviewContext {
  spaceId: string;
  chatId: string;
  managerSystemPrompt: string;
  stickyDigests: string[];
  recentHistory: string[];
  fileContexts: FileContextMetaData[];
  focus: AttentionFocus;
}

export interface TelemetryEvent {
  type: 'telemetry_event';
  channel: string;
  id: string;
  eventType: string;
  timestamp: Temporal.Instant; // Mapped to native Temporal API
  message: string;
  summary: string;
  payload: Record<string, unknown>;
}

export interface BranchTopology {
  parentBranch: string;
  forkPoint: string;
}

export type AppEvent =
  | { type: 'chat_stream'; text: string }
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
      activeBranch: string;
      branches: Record<string, BranchTopology>;
    }
  | { type: 'preview_context'; context: PreviewContext }
  | { type: 'agent_stream'; agentId: number; text: string }
  | { type: 'streamed_graph_event'; event: GraphEvent }
  | TelemetryEvent;

export enum DelegationStrategy {
  SKIP = 1,
  MANUAL = 2,
  REVIEW = 3,
  REFINE = 4,
}