import { ComponentFixture, TestBed } from '@angular/core/testing';
import { FlowTrackerComponent, FlowView } from './flow-tracker.component';
import { ComponentRef } from '@angular/core';
import { describe, beforeEach, it, expect } from 'vitest';

describe('FlowTrackerComponent', () => {
  let component: FlowTrackerComponent;
  let fixture: ComponentFixture<FlowTrackerComponent>;
  let componentRef: ComponentRef<FlowTrackerComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [FlowTrackerComponent]
    }).compileComponents();

    fixture = TestBed.createComponent(FlowTrackerComponent);
    component = fixture.componentInstance;
    componentRef = fixture.componentRef;
    
    const mockMap = new Map<string, FlowView>();
    mockMap.set('flow-1', {
      flowId: 'flow-1',
      taskId: 'task-1',
      agentCount: 1,
      status: 'running',
      agents: new Map([
        ['agent-1', {
          agentId: 'agent-1',
          agentIndex: 1,
          instruction: 'Write a loop',
          status: 'running_tests',
          attempt: 1,
          trace: '',
          passed: false
        }]
      ])
    });

    componentRef.setInput('flows', mockMap);
    fixture.detectChanges();
  });

  it('should format status strings cleanly', () => {
    expect(component.formatStatus('executing_instructions')).toBe('Executing Instructions');
  });

  it('should render the active flows', () => {
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.textContent).toContain('Flow: 1');
    expect(compiled.textContent).toContain('Agent 1');
    expect(compiled.textContent).toContain('Running Tests');
  });

  it('should return correct colors based on agent state', () => {
    expect(component.getAgentColor({ passed: true } as any)).toBe('#2b8a3e');
    expect(component.getAgentColor({ trace: 'error' } as any)).toBe('#e03131');
    expect(component.getAgentColor({ status: 'running_tests' } as any)).toBe('#339af0');
  });
});