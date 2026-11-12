import { ComponentFixture, TestBed } from '@angular/core/testing';
import { FlowGraphComponent } from './flow-graph.component';
import { GraphStore, ChatConnectionService, ThinkspaceStateService, ChatItem } from '@org/llm-state-chat';
import { signal, WritableSignal } from '@angular/core';
import { describe, beforeEach, it, expect, vi, Mock } from 'vitest';

interface MockGraphStore {
  projections: WritableSignal<Map<string, string[]>>;
  nodes: WritableSignal<Map<string, ChatItem>>;
}

interface MockThinkspaceState {
  activeSpaceId: WritableSignal<string>;
  activeChatId: WritableSignal<string>;
}

describe('FlowGraphComponent', () => {
  let component: FlowGraphComponent;
  let fixture: ComponentFixture<FlowGraphComponent>;
  let mockGraphStore: MockGraphStore;
  let mockWorkspace: MockThinkspaceState;
  let connectionLoadSpy: Mock<(graphId: string) => void>;

  const mockTime = (s: string) => ({ toString: () => s }) as unknown as Temporal.Instant;

  beforeEach(async () => {
    mockGraphStore = {
      projections: signal(new Map()),
      nodes: signal(new Map())
    };

    mockWorkspace = {
      activeSpaceId: signal('golang'),
      activeChatId: signal('chat-1')
    };

    connectionLoadSpy = vi.fn();

    await TestBed.configureTestingModule({
      imports: [FlowGraphComponent],
      providers: [
        { provide: GraphStore, useValue: mockGraphStore },
        { provide: ThinkspaceStateService, useValue: mockWorkspace },
        { provide: ChatConnectionService, useValue: { loadSubGraph: connectionLoadSpy } }
      ]
    }).compileComponents();

    fixture = TestBed.createComponent(FlowGraphComponent);
    component = fixture.componentInstance;

    fixture.componentRef.setInput('flowId', 'flow-123');
  });

  it('should trigger loadSubGraph on init', () => {
    fixture.detectChanges();
    expect(connectionLoadSpy).toHaveBeenCalledWith('flow-123');
  });

  it('should resolve cross-boundary parent (Manager Node) via the global cache', () => {
    const managerNode: ChatItem = {
      id: 'manager-1',
      type: 'model',
      content: 'Plan',
      timestamp: mockTime('2026-11-12T10:00:00Z'),
      tombstoned: false
    };
    const taskNode: ChatItem = {
      id: 'task-1',
      type: 'prompt',
      content: 'Agent Work',
      edges: { parent_id: 'manager-1' },
      timestamp: mockTime('2026-11-12T10:00:01Z'),
      tombstoned: false
    };

    const cache = new Map<string, ChatItem>();
    cache.set(managerNode.id, managerNode);
    cache.set(taskNode.id, taskNode);
    mockGraphStore.nodes.set(cache);

    const projections = new Map<string, string[]>();
    projections.set('golang:chat-1:flow-123', ['task-1']);
    mockGraphStore.projections.set(projections);

    fixture.detectChanges();

    const layout = component.layout();
    expect(layout.nodes.length).toBe(2);
    expect(layout.nodes.some(n => n.id === 'manager-1')).toBe(true);
    expect(layout.nodes.some(n => n.id === 'task-1')).toBe(true);
    expect(layout.edges.length).toBe(1);
    expect(layout.edges[0].id).toBe('manager-1->task-1');
  });

  it('should resolve cross-boundary bridge (Options Node) and compute diamond convergence depth', () => {
    const managerNode: ChatItem = {
      id: 'manager-1',
      type: 'model',
      content: 'Plan',
      timestamp: mockTime('2026-11-12T10:00:00Z'),
      tombstoned: false
    };
    const task1: ChatItem = {
      id: 'task-1',
      type: 'prompt',
      content: 'Branch 1',
      edges: { parent_id: 'manager-1' },
      timestamp: mockTime('2026-11-12T10:00:01Z'),
      tombstoned: false
    };
    const task2: ChatItem = {
      id: 'task-2',
      type: 'prompt',
      content: 'Branch 2',
      edges: { parent_id: 'manager-1' },
      timestamp: mockTime('2026-11-12T10:00:01Z'),
      tombstoned: false
    };
    const retryTask2: ChatItem = {
      id: 'task-2-retry',
      type: 'prompt',
      content: 'Branch 2 Retry',
      edges: { parent_id: 'task-2' },
      timestamp: mockTime('2026-11-12T10:00:02Z'),
      tombstoned: false
    };
    const joinNode: ChatItem = {
      id: 'join-1',
      type: 'flow_execution',
      content: 'Merged',
      edges: { parent_0: 'task-1', parent_1: 'task-2-retry' },
      timestamp: mockTime('2026-11-12T10:00:03Z'),
      tombstoned: false
    };
    const optionsNode: ChatItem = {
      id: 'options-1',
      type: 'system',
      content: 'Awaiting strategy',
      edges: { parent_id: 'join-1' },
      timestamp: mockTime('2026-11-12T10:00:04Z'),
      tombstoned: false
    };
    const synthPrompt: ChatItem = {
      id: 'synth-1',
      type: 'prompt',
      content: 'Synthesize',
      edges: { parent_id: 'options-1' },
      timestamp: mockTime('2026-11-12T10:00:05Z'),
      tombstoned: false
    };

    const cache = new Map<string, ChatItem>();
    [managerNode, task1, task2, retryTask2, joinNode, optionsNode, synthPrompt].forEach(n =>
      cache.set(n.id, n)
    );
    mockGraphStore.nodes.set(cache);

    const projections = new Map<string, string[]>();
    projections.set('golang:chat-1:flow-123', ['task-1', 'task-2', 'task-2-retry', 'join-1', 'synth-1']);
    mockGraphStore.projections.set(projections);

    fixture.detectChanges();

    const layout = component.layout();
    expect(layout.nodes.length).toBe(7);
    expect(layout.edges.length).toBe(7);

    const retryLayout = layout.nodes.find(n => n.id === 'task-2-retry')!;
    const joinLayout = layout.nodes.find(n => n.id === 'join-1')!;
    const optionsLayout = layout.nodes.find(n => n.id === 'options-1')!;
    const synthLayout = layout.nodes.find(n => n.id === 'synth-1')!;

    // Join node must sit strictly below its deepest parent (retryTask2)
    expect(joinLayout.y).toBeGreaterThan(retryLayout.y);
    expect(optionsLayout.y).toBeGreaterThan(joinLayout.y);
    expect(synthLayout.y).toBeGreaterThan(optionsLayout.y);
  });
});