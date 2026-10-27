export interface BaseChatItem {
  id: string;
  content: string;
  tags?: string[];
  metadata?: Record<string, string>;
  edges?: Record<string, string>; // The new generic topological mapping (e.g., amended_by, forked_from)
}

export interface UserChatItem extends BaseChatItem {
  type: 'user';
}

export interface ModelChatItem extends BaseChatItem {
  type: 'model' | 'system';
}

export interface FlowCardChatItem extends BaseChatItem {
  type: 'flow_card';
  flowId: string;
  status: 'running' | 'completed';
}

export interface FileDeltaChatItem extends BaseChatItem {
  type: 'file_delta';
  path: string;
  previewSnippet: string;
  fileRef: string;
}

export type ChatItem = 
  | UserChatItem 
  | ModelChatItem 
  | FlowCardChatItem 
  | FileDeltaChatItem;

export interface LogItem {
  id: string;
  timestamp: number;
  level: string;
  message: string;
}

export interface TimelineEntry {
  id: string;
  timestamp: number;
  message: string;
  isError: boolean;
}

export interface FlowAgentState {
  agentId: string;
  agentIndex: number;
  instruction: string;
  status: string;
  attempt: number;
  trace: string;
  passed: boolean;
  timeline: TimelineEntry[];
}

export interface FlowState {
  flowId: string;
  taskId: string;
  agentCount: number;
  completedCount: number;
  status: 'running' | 'completed';
  agents: Map<string, FlowAgentState>;
}