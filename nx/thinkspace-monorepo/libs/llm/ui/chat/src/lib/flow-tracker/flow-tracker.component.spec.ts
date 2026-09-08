import { ComponentFixture, TestBed } from '@angular/core/testing';
import { FlowTrackerComponent } from './flow-tracker.component';
import { ComponentRef } from '@angular/core';
import { describe, beforeEach, it, expect } from 'vitest';
import { FlowState } from '@org/llm-state-chat';

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
    
    const mockMap = new Map<string, FlowState>();
    mockMap.set('flow-1', {
      flowId: 'flow-1',
      taskId: 'task-1',
      agentCount: 1,
      completedCount: 0,
      status: 'running',
      agents: new Map([
        ['agent-1', {
          agentId: 'agent-1',
          agentIndex: 1,
          instruction: 'Write a loop',
          status: 'running_tests',
          attempt: 1,
          trace: '',
          passed: false,
          timeline: [
            { id: '1', timestamp: Date.now(), message: '[Attempt 1] Status: running_tests', isError: false }
          ]
        }]
      ])
    });

    componentRef.setInput('flows', mockMap);
    fixture.detectChanges();
  });

  it('should compute flowsArray sorted by running status', () => {
    const flows = component.flowsArray();
    expect(flows.length).toBe(1);
    expect(flows[0].flowId).toBe('flow-1');
    expect(flows[0].agentsArray.length).toBe(1);
    expect(flows[0].agentsArray[0].agentId).toBe('agent-1');
  });

  it('should render the active flows as a terminal feed', () => {
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.textContent).toContain('Flow: 1');
    expect(compiled.textContent).toContain('Agent 1');
    expect(compiled.textContent).toContain('Write a loop');
    expect(compiled.textContent).toContain('[Attempt 1] Status: running_tests');
  });
});