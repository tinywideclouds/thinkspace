import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ChatFeedComponent } from './chat-feed.component';
import { ComponentRef } from '@angular/core';
import { describe, beforeEach, it, expect } from 'vitest';
import { FlowState } from '@org/llm-state-chat';

describe('ChatFeedComponent', () => {
  let component: ChatFeedComponent;
  let fixture: ComponentFixture<ChatFeedComponent>;
  let componentRef: ComponentRef<ChatFeedComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ChatFeedComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(ChatFeedComponent);
    component = fixture.componentInstance;
    componentRef = fixture.componentRef;

    componentRef.setInput('feed', [
      { id: '1', source: 'user', content: 'Test prompt' },
      { id: '2', source: 'model', content: 'Test response' },
      { id: '3', source: 'flow_card', content: 'Flow Card Test', flowId: 'f1' },
    ]);

    const mockStates = new Map<string, FlowState>();
    mockStates.set('f1', {
      flowId: 'f1',
      taskId: 't1',
      agentCount: 1,
      completedCount: 0,
      status: 'running',
      agents: new Map(),
    });

    componentRef.setInput('flowStates', mockStates);
    fixture.detectChanges();
  });

  it('should create the component', () => {
    expect(component).toBeTruthy();
  });

  it('should render feed items and pass summary to flow card', () => {
    const compiled = fixture.nativeElement as HTMLElement;
    const strongTags = compiled.querySelectorAll('strong');

    expect(strongTags.length).toBe(3);
    expect(strongTags[0].textContent).toContain('user');
    expect(strongTags[1].textContent).toContain('model');
    // Third strong tag is inside the flow card, verifying the summary was passed
    expect(strongTags[2].textContent).toContain('Flow Card Test');
  });
});
