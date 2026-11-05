import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ChatFlowCardComponent } from './chat-flow-card.component';
import { ComponentRef } from '@angular/core';
import { vi, describe, beforeEach, it, expect } from 'vitest';
import { FlowExecutionChatItem } from '@org/llm-state-chat';

describe('ChatFlowCardComponent', () => {
  let component: ChatFlowCardComponent;
  let fixture: ComponentFixture<ChatFlowCardComponent>;
  let componentRef: ComponentRef<ChatFlowCardComponent>;

  const mockItem: FlowExecutionChatItem = {
    id: '1',
    type: 'flow_execution',
    timestamp: { toString: () => '2026-10-23T12:00:00Z' } as unknown as Temporal.Instant,
    flowId: 'flow-123',
    taskId: 'task-123',
    task: 'I am assigning 2 agents to explore adding JWT authentication.',
    status: 'COMPLETED',
    error: '',
    content: 'Successfully implemented JWT authentication.',
    agents: [
      { agentId: 'agent-1', passed: true, instruction: 'Do A', rawPayload: '', verificationTrace: '', stateDelta: '' }
    ]
  };

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ChatFlowCardComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(ChatFlowCardComponent);
    component = fixture.componentInstance;
    componentRef = fixture.componentRef;

    componentRef.setInput('item', mockItem);
    fixture.detectChanges();
  });

  it('should display the fixed task header and durable summary', () => {
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.textContent).toContain('Task: I am assigning 2 agents to explore adding JWT authentication.');
    expect(compiled.textContent).toContain('Successfully implemented JWT authentication.');
  });

  it('should display the error root cause when status is FAILED and error is present', () => {
    const errorItem = { 
      ...mockItem, 
      status: 'FAILED', 
      error: 'The verification step failed because DB_MAX_IDLE is undefined.' 
    };
    componentRef.setInput('item', errorItem);
    fixture.detectChanges();

    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.textContent).toContain('Root Cause Analysis');
    expect(compiled.textContent).toContain('The verification step failed because DB_MAX_IDLE is undefined.');
  });

  it('should not display the error block if status is not FAILED', () => {
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.textContent).not.toContain('Root Cause Analysis');
  });

  it('should emit inspect event with the full flow item on click', () => {
    const emitSpy = vi.spyOn(component.inspect, 'emit');
    const button = fixture.nativeElement.querySelector('button') as HTMLButtonElement;

    button.click();

    expect(emitSpy).toHaveBeenCalledWith(mockItem);
  });
});