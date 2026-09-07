export interface DomainFlowEvent {
  type: 'flow_event';
  flowId: string;
  eventType: string;
  timestamp: string;
  taskId: string;
  agentCount: number;
  agentId: string;
  agentIndex: number;
  instruction: string;
  status: string;
  attempt: number;
  trace: string;
  candidateId: string;
  passed: boolean;
}

// Added the standalone SpaceInfo interface
export interface SpaceInfo {
  id: string;
  name: string;
}

export type DomainEvent =
  | { type: 'chat_stream'; text: string }
  | { type: 'log_message'; level: string; message: string }
  | { type: 'delegation_start'; agentCount: number; instructions: string }
  | { type: 'delegation_complete'; summary: string }
  | { type: 'agent_start'; agentId: number; instructions: string }
  | { type: 'agent_stream'; agentId: number; text: string }
  | { type: 'agent_complete'; agentId: number; branch: string; verified: boolean }
  | { type: 'request_strategy'; active: boolean }
  | { type: 'request_review'; branch: string }
  | { type: 'available_spaces'; spaces: SpaceInfo[] } // Updated to use the interface
  | DomainFlowEvent;

export enum DomainDelegationStrategy {
  SKIP = 1,
  MANUAL = 2,
  REVIEW = 3,
  REFINE = 4,
}