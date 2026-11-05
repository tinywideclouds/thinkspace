import { ComponentFixture, TestBed } from '@angular/core/testing';
import { FlowInspectorComponent } from './flow-inspector.component';
import { ComponentRef } from '@angular/core';
import { vi, describe, beforeEach, it, expect } from 'vitest';
import { FlowExecutionChatItem } from '@org/llm-state-chat';

describe('FlowInspectorComponent', () => {
  let component: FlowInspectorComponent;
  let fixture: ComponentFixture<FlowInspectorComponent>;
  let componentRef: ComponentRef<FlowInspectorComponent>;

  const mockItem: FlowExecutionChatItem = {
    id: 'exec-1',
    type: 'flow_execution',
    timestamp: { toString: () => '2026-10-23T12:00:00Z' } as unknown as Temporal.Instant,
    flowId: 'flow-999',
    taskId: 'task-1',
    task: 'Test Task',
    content: 'Task Summary',
    status: 'COMPLETED',
    error: '',
    agents: [
      { agentId: 'agent-1', passed: true, instruction: 'Do A', rawPayload: 'Output A', verificationTrace: 'Trace A', stateDelta: 'Delta A' },
      { agentId: 'agent-2', passed: false, instruction: 'Do B', rawPayload: 'Output B', verificationTrace: 'Trace B', stateDelta: 'Delta B' }
    ]
  };

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [FlowInspectorComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(FlowInspectorComponent);
    component = fixture.componentInstance;
    componentRef = fixture.componentRef;

    componentRef.setInput('flowItem', mockItem);
    fixture.detectChanges();
  });

  it('should initialize and set the first agent as active', () => {
    const item = component.flowItem();

    expect(item).toBeTruthy();
    expect(item.flowId).toBe('flow-999');
    expect(item.agents.length).toBe(2);

    expect(component.selectedAgentId()).toBe('agent-1');
    expect(component.activeAgent()?.instruction).toBe('Do A');
  });

  it('should change active agent when selected', () => {
    component.selectedAgentId.set('agent-2');
    fixture.detectChanges();

    expect(component.activeAgent()?.instruction).toBe('Do B');
    expect(component.activeAgent()?.passed).toBe(false);
  });

  it('should emit close event when close button is clicked', () => {
    const emitSpy = vi.spyOn(component.inspectorClosed, 'emit');
    const closeButton = fixture.nativeElement.querySelector('button') as HTMLButtonElement;
    
    closeButton.click();
    expect(emitSpy).toHaveBeenCalled();
  });
});