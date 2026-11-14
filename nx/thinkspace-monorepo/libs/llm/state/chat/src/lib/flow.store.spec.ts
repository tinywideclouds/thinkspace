import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting } from '@angular/common/http/testing';
import { FlowStore } from './flow.store';
import { describe, beforeEach, it, expect } from 'vitest';

describe('FlowStore', () => {
  let store: FlowStore;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [FlowStore, provideHttpClient(), provideHttpClientTesting()]
    });
    store = TestBed.inject(FlowStore);
  });

  it('should process telemetry events and update flow states independently', () => {
    store.processTelemetry({
      type: 'telemetry_event',
      channel: 'manager',
      id: '1',
      eventType: 'flow_start',
      timestamp: Temporal.Now.instant(),
      message: '',
      summary: '',
      payload: { type: 'flow', flowId: 'f1', taskId: 't1', agentCount: 1 }
    });

    expect(store.flowStates().has('f1')).toBe(true);
    expect(store.flowStates().get('f1')?.status).toBe('running');

    store.processTelemetry({
      type: 'telemetry_event',
      channel: 'agent-1',
      id: '2',
      eventType: 'agent_status',
      timestamp: Temporal.Now.instant(),
      message: 'Executing instructions...',
      summary: '',
      payload: {
        type: 'agent',
        flowId: 'f1',
        agentId: 'agent-1',
        agentIndex: 1,
        instruction: 'Do task',
        attempt: 1,
        trace: '',
        candidateId: '',
        passed: false
      }
    });

    expect(store.flowStates().get('f1')?.agents.has('agent-1')).toBe(true);
    expect(store.flowStates().get('f1')?.agents.get('agent-1')?.status).toBe('Executing instructions...');

    store.processTelemetry({
      type: 'telemetry_event',
      channel: 'manager',
      id: '3',
      eventType: 'flow_complete',
      timestamp: Temporal.Now.instant(),
      message: '',
      summary: '',
      payload: { type: 'flow', flowId: 'f1', taskId: 't1', agentCount: 1 }
    });

    expect(store.flowStates().get('f1')?.status).toBe('completed');
  });

  it('should clear logs when resetTurn is called and retainLogs is false', () => {
    store.appendSystemLog('INFO', 'Test');
    store.retainLogs.set(false);
    store.resetTurn();
    expect(store.systemLogs().length).toBe(0);
  });
});