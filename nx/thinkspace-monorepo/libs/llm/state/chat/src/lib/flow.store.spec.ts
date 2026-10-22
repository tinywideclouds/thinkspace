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
      type: 'telemetry_event', channel: 'manager', id: '1', eventType: 'flow_start', timestamp: '', message: '', summary: '',
      payload: { flow_id: 'f1', agent_count: 1 }
    });

    expect(store.flowStates().has('f1')).toBe(true);
    expect(store.flowStates().get('f1')?.status).toBe('running');

    store.processTelemetry({
      type: 'telemetry_event', channel: 'manager', id: '2', eventType: 'flow_complete', timestamp: '', message: '', summary: '',
      payload: { flow_id: 'f1', agent_index: 1, passed: true }
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