import { ComponentFixture, TestBed } from '@angular/core/testing';
import { LinearFeedComponent } from './linear-feed.component';
import { LinearFeedStore } from '../linear-feed.store';
import { signal } from '@angular/core';
import { describe, beforeEach, it, expect, vi } from 'vitest';

describe('LinearFeedComponent', () => {
  let component: LinearFeedComponent;
  let fixture: ComponentFixture<LinearFeedComponent>;
  let mockStore: any;

  beforeEach(async () => {
    mockStore = {
      visibleFeed: signal([
        { id: '1', source: 'user', content: 'Test prompt' },
        { id: '2', source: 'model', content: 'Test response' },
        { id: '3', source: 'flow_card', content: 'Flow Card Test', flowId: 'f1' }
      ]),
      flowStates: signal(new Map([
        ['f1', { flowId: 'f1', taskId: 't1', agentCount: 1, completedCount: 0, status: 'running', agents: new Map() }]
      ])),
      inspectFlow: vi.fn()
    };

    await TestBed.configureTestingModule({
      imports: [LinearFeedComponent],
      providers: [
        { provide: LinearFeedStore, useValue: mockStore }
      ]
    }).compileComponents();

    fixture = TestBed.createComponent(LinearFeedComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create the component', () => {
    expect(component).toBeTruthy();
  });

  it('should render user and model messages', () => {
    const compiled = fixture.nativeElement as HTMLElement;
    const strongTags = compiled.querySelectorAll('strong');
    
    expect(strongTags.length).toBe(3);
    expect(strongTags[0].textContent?.trim()).toBe('user');
    expect(strongTags[1].textContent?.trim()).toBe('model');
    expect(compiled.textContent).toContain('Test prompt');
  });

  it('should pass summary to flow card and trigger inspect on click', () => {
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.textContent).toContain('Flow Card Test');

    // Simulate clicking the inspect button inside the flow card (which emits up to the feed component)
    const inspectButton = compiled.querySelector('button');
    if (inspectButton) {
      inspectButton.click();
      expect(mockStore.inspectFlow).toHaveBeenCalledWith('f1');
    }
  });
});