import { TestBed } from '@angular/core/testing';
import { TimelineStore } from './timeline.store';
import { DomainGraphEvent, DomainBranchTopology } from '@org/llm-core-facade';
import { describe, beforeEach, it, expect } from 'vitest';

describe('TimelineStore', () => {
  let store: TimelineStore;

  beforeEach(() => {
    TestBed.configureTestingModule({ providers: [TimelineStore] });
    store = TestBed.inject(TimelineStore);
  });

  it('should process sync history with topology data and map edges', () => {
    const mockEvents: DomainGraphEvent[] = [
      { 
        id: '1', 
        type: 'model', 
        content: 'test msg', 
        timestamp: "", 
        metadata: {}, 
        edges: { amended_by: '2' },
        branchId: 'feature-branch' 
      }
    ];
    
    const mockTopology: Record<string, DomainBranchTopology> = {
      'feature-branch': { parentBranch: 'main', forkPoint: '0' }
    };
    
    store.processSyncHistory(mockEvents, 'next-cursor-123', true, 'feature-branch', mockTopology, 50);
    
    expect(store.coreChat().length).toBe(1);
    expect(store.coreChat()[0].edges?.['amended_by']).toBe('2'); // Edge mapping verified
    expect(store.activeBranch()).toBe('feature-branch');
    expect(store.branches()['feature-branch'].parentBranch).toBe('main');
    expect(store.nextCursor()).toBe('next-cursor-123');
    expect(store.hasMoreHistory()).toBe(true);
    expect(store.totalHistoryMessages()).toBe(50);
    expect(store.isLoadingHistory()).toBe(false);
  });

  it('should strictly filter live ingest events not matching the active branch', () => {
    store.activeBranch.set('main');
    
    const crossBranchEvent: DomainGraphEvent = {
      id: '2', type: 'model', content: 'cross contamination', timestamp: '', metadata: {}, edges: {}, branchId: 'other-branch'
    };
    
    const correctBranchEvent: DomainGraphEvent = {
      id: '3', type: 'model', content: 'correct branch', timestamp: '', metadata: {}, edges: {}, branchId: 'main'
    };

    store.ingestEvent(crossBranchEvent);
    expect(store.coreChat().length).toBe(0); // Should be ignored

    store.ingestEvent(correctBranchEvent);
    expect(store.coreChat().length).toBe(1); // Should be ingested
    expect(store.coreChat()[0].content).toBe('correct branch');
  });

  it('should clear session and topology', () => {
    store.activeBranch.set('feature-branch');
    store.branches.set({ 'feature-branch': { parentBranch: 'main', forkPoint: '1' } });
    
    store.clearSession();
    
    expect(store.activeBranch()).toBe('main');
    expect(store.branches()).toEqual({});
  });
});