export interface SpaceInfo {
  id: string;
  name: string;
}

export interface DomainGraphEvent {
  id: string;
  timestamp: string;
  type: string;
  content: string;
  metadata: Record<string, string>;
}

export interface DomainDigestMeta {
  id: string;
  summary: string;
  isSticky: boolean;
}

export interface FileContextMetaData {
  path: string;
  level: string;
  tokenCount: number;
}

export interface DomainAttentionFocus {
  primaryFocus: string[];
  relatedContext: string[];
  dormantLenses: string[];
}

export interface DomainPreviewContext {
  spaceId: string;
  chatId: string;
  managerSystemPrompt: string;
  stickyDigests: string[];
  recentHistory: string[];
  fileContexts: FileContextMetaData[];
  focus: DomainAttentionFocus;
}

export interface DomainTelemetryEvent {
  type: 'telemetry_event';
  channel: string;
  id: string;
  eventType: string;
  timestamp: string;
  message: string;
  summary: string;
  payload: any;
}

export type DomainEvent =
  | { type: 'chat_stream'; text: string }
  | { type: 'request_strategy'; active: boolean }
  | { type: 'request_review'; branch: string }
  | { type: 'request_transition'; targetDomain: string; reasoning: string }
  | { type: 'available_spaces'; spaces: SpaceInfo[] }
  | { type: 'sync_history'; recentEvents: DomainGraphEvent[]; digests: Record<string, DomainDigestMeta>; focus: DomainAttentionFocus; nextCursor: string; hasMore: boolean }
  | { type: 'preview_context'; context: DomainPreviewContext }
  | { type: 'agent_stream'; agentId: number; text: string }
  | { type: 'streamed_graph_event'; event: DomainGraphEvent }
  | DomainTelemetryEvent;

export enum DomainDelegationStrategy {
  SKIP = 1,
  MANUAL = 2,
  REVIEW = 3,
  REFINE = 4,
}