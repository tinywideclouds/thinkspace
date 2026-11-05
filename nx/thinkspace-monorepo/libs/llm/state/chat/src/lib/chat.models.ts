import { AgentRecord } from '@org/llm-core-facade';

export type ChatItemType = 'prompt' | 'model' | 'system' | 'file_delta' | 'flow_execution';

export interface BaseChatItem {
  id: string;
  type: ChatItemType;
  timestamp: Temporal.Instant;
  tags?: string[];
  metadata?: Record<string, string>;
  edges?: Record<string, string>;
  branchId?: string;
}

export interface MessageChatItem extends BaseChatItem {
  type: 'prompt' | 'model' | 'system';
  content: string;
}

export interface FlowExecutionChatItem extends BaseChatItem {
  type: 'flow_execution';
  content: string; // summary
  flowId: string;
  taskId: string;
  task: string;
  status: 'UNSPECIFIED' | 'RUNNING' | 'COMPLETED' | 'FAILED' | string;
  error: string;
  agents: AgentRecord[];
}

export interface FileDeltaChatItem extends BaseChatItem {
  type: 'file_delta';
  content: string; // summary
  path: string;
  previewSnippet: string;
  fileRef: string;
  parentId?: string;
}

export type ChatItem = 
  | MessageChatItem 
  | FlowExecutionChatItem 
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