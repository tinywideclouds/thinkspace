export interface SpaceInfo {
  id: string;
  name: string;
}

export interface DomainLedgerEvent {
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

export interface DomainPreviewContext {
  spaceId: string;
  chatId: string;
  managerSystemPrompt: string;
  stickyDigests: string[];
  recentHistory: string[];
  fileContexts: FileContextMetaData[];
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
  | { type: 'available_spaces'; spaces: SpaceInfo[] }
  | { type: 'sync_history'; recentEvents: DomainLedgerEvent[]; digests: Record<string, DomainDigestMeta> }
  | { type: 'preview_context'; context: DomainPreviewContext }
  | { type: 'agent_stream'; agentId: number; text: string }
  | DomainTelemetryEvent;

export enum DomainDelegationStrategy {
  SKIP = 1,
  MANUAL = 2,
  REVIEW = 3,
  REFINE = 4,
}