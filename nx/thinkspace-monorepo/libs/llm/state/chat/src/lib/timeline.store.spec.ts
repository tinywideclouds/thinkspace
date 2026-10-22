import { TestBed } from '@angular/core/testing';
import { TimelineStore } from './timeline.store';
import { describe, beforeEach, it, expect } from 'vitest';

describe('TimelineStore', () => {
  let store: TimelineStore;

  beforeEach(() => {
    TestBed.configureTestingModule({ providers: [TimelineStore] });
    store = TestBed.inject(TimelineStore);
  });

  it('should stream model text and concatenate cleanly', () => {
    store.appendToLastModelMessage('Hello');
    store.appendToLastModelMessage(' World');
    
    expect(store.coreChat().length).toBe(1);
    expect(store.coreChat()[0].source).toBe('model');
    expect(store.coreChat()[0].content).toBe('Hello World');
  });

  it('should clear thinking indicators on command', () => {
    store.appendCoreChat('system', 'Manager Thinking...');
    expect(store.coreChat().length).toBe(1);
    
    store.clearThinkingIndicators();
    expect(store.coreChat().length).toBe(0);
  });
});