import { TestBed } from '@angular/core/testing';
import { TimelineStore } from './timeline.store';
import { GraphEvent } from '@org/llm-core-facade';
import { describe, beforeEach, afterEach, it, expect, vi } from 'vitest';

describe('TimelineStore', () => {
  let store: TimelineStore;

  beforeEach(() => {
    vi.stubGlobal('Temporal', {
      Now: { instant: () => ({ toString: () => '2026-10-23T12:00:00Z' }) },
      Instant: { from: (s: string) => ({ toString: () => s }) }
    });

    TestBed.configureTestingModule({
      providers: [TimelineStore],
    });
    store = TestBed.inject(TimelineStore);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('should initialize with empty state', () => {
    expect(store.coreChat()).toEqual([]);
    expect(store.hasMoreHistory()).toBe(false);
    expect(store.activeBranch()).toBe('main');
  });

  it('should process sync history and strictly map to ChatItem', () => {
    const events: GraphEvent[] = [
      {
        id: 'msg-1',
        type: 'prompt',
        timestamp: Temporal.Instant.from('2026-10-23T12:00:00Z'),
        tags: [],
        content: 'hello',
        tombstoned: false,
        metadata: {},
        edges: {}
      },
      {
        id: 'file-1',
        type: 'file_delta',
        timestamp: Temporal.Instant.from('2026-10-23T12:01:00Z'),
        tags: [],
        content: 'updated file',
        path: 'test.go',
        previewSnippet: 'func main()',
        fileRef: 'ref-1',
        tombstoned: false,
        metadata: {},
        edges: {}
      }
    ];

    store.processSyncHistory(events, 'next-cursor', true, 'main', {}, 2);

    const chat = store.coreChat();
    expect(chat.length).toBe(2);
    expect(chat[0].type).toBe('prompt');
    expect(chat[0].timestamp.toString()).toBe('2026-10-23T12:00:00Z');
    
    const fileItem = chat[1];
    expect(fileItem.type).toBe('file_delta');
    if (fileItem.type === 'file_delta') {
      expect(fileItem.path).toBe('test.go');
    }
    
    expect(store.hasMoreHistory()).toBe(true);
    expect(store.nextCursor()).toBe('next-cursor');
  });

  it('should strictly map and ingest a new GraphEvent with flow execution data', () => {
    const event: GraphEvent = {
      id: 'exec-1',
      type: 'flow_execution',
      timestamp: Temporal.Instant.from('2026-10-23T12:02:00Z'),
      tags: [],
      content: 'Running tests',
      flowId: 'flow-123',
      taskId: 'task-123',
      task: 'Build it',
      status: 'RUNNING',
      error: '',
      agents: [
        {
          agentId: 'agent-1',
          passed: false,
          instruction: 'do work',
          rawPayload: '{}',
          verificationTrace: '',
          stateDelta: ''
        }
      ],
      tombstoned: false,
      metadata: {},
      edges: {}
    };

    store.ingestEvent(event);
    
    const item = store.coreChat()[0];
    expect(item.type).toBe('flow_execution');
    if (item.type === 'flow_execution') {
      expect(item.flowId).toBe('flow-123');
      expect(item.status).toBe('RUNNING');
      expect(item.agents.length).toBe(1);
      expect(item.agents[0].agentId).toBe('agent-1');
    }
  });

  it('should upsert an existing GraphEvent when ingested with the same ID', () => {
    const event1: GraphEvent = {
      id: 'exec-upsert',
      type: 'flow_execution',
      timestamp: Temporal.Instant.from('2026-10-23T12:02:00Z'),
      tags: [],
      content: 'Running',
      flowId: 'flow-123',
      taskId: 'task-123',
      task: 'Build it',
      status: 'RUNNING',
      error: '',
      agents: [],
      tombstoned: false,
      metadata: {},
      edges: {}
    };

    const event2: GraphEvent = {
      id: 'exec-upsert', // Same ID!
      type: 'flow_execution',
      timestamp: Temporal.Instant.from('2026-10-23T12:02:05Z'),
      tags: [],
      content: 'Done',
      flowId: 'flow-123',
      taskId: 'task-123',
      task: 'Build it',
      status: 'COMPLETED',
      error: '',
      agents: [],
      tombstoned: false,
      metadata: {},
      edges: {}
    };

    store.ingestEvent(event1);
    expect(store.coreChat().length).toBe(1);
    expect((store.coreChat()[0] as any).status).toBe('RUNNING');

    store.ingestEvent(event2);
    expect(store.coreChat().length).toBe(1); // Should UPSERT, not append
    expect((store.coreChat()[0] as any).status).toBe('COMPLETED');
  });

  it('should remove a node entirely if ingested with tombstoned: true', () => {
    const event: GraphEvent = {
      id: 'msg-delete',
      type: 'prompt',
      timestamp: Temporal.Instant.from('2026-10-23T12:00:00Z'),
      tags: [],
      content: 'hello',
      tombstoned: false,
      metadata: {},
      edges: {}
    };

    store.ingestEvent(event);
    expect(store.coreChat().length).toBe(1);

    const deleteEvent = { ...event, tombstoned: true };
    store.ingestEvent(deleteEvent);
    expect(store.coreChat().length).toBe(0);
  });

  it('should ignore events from inactive branches', () => {
    store.activeBranch.set('main');
    
    const event: GraphEvent = {
      id: 'msg-1',
      type: 'model',
      timestamp: Temporal.Instant.from('2026-10-23T12:02:00Z'),
      tags: [],
      content: 'I am on another branch',
      branchId: 'feature-branch',
      tombstoned: false,
      metadata: {},
      edges: {}
    };

    store.ingestEvent(event);
    expect(store.coreChat().length).toBe(0);
  });

  it('should clear thinking indicators when appending new chat via standard O(1) map key deletion', () => {
    store.appendCoreChat('system', 'Manager Thinking...');
    expect(store.coreChat().length).toBe(1);

    store.appendCoreChat('prompt', 'Actual prompt');
    expect(store.coreChat().length).toBe(1); // Replaced the thinking indicator
    expect(store.coreChat()[0].type).toBe('prompt');
  });
});