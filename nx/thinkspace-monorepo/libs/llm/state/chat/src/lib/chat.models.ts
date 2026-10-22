export interface ChatItem {
  id: string;
  source: 'user' | 'model' | 'system' | 'flow_card';
  content: string;
  flowId?: string;
  status?: 'running' | 'completed'; 
  tags?: string[];
}

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