import { ComponentFixture, TestBed } from '@angular/core/testing';
import { FlowInspectorComponent } from './flow-inspector.component';
import { ComponentRef } from '@angular/core';
import { vi, describe, beforeEach, it, expect } from 'vitest';

const mockXml = `<?xml version="1.0" encoding="UTF-8"?>
<FlowReceipt FlowID="flow-999" TaskID="task-1" Timestamp="2026-09-02T12:00:00Z">
  <Task>Test Task</Task>
  <Summary>Task Summary</Summary>
  <Agents>
    <Agent AgentID="agent-1" Passed="true">
      <Instruction>Do A</Instruction>
      <RawPayload>Output A</RawPayload>
      <VerificationTrace>Trace A</VerificationTrace>
      <StateDelta>Delta A</StateDelta>
    </Agent>
    <Agent AgentID="agent-2" Passed="false">
      <Instruction>Do B</Instruction>
      <RawPayload>Output B</RawPayload>
      <VerificationTrace>Trace B</VerificationTrace>
      <StateDelta>Delta B</StateDelta>
    </Agent>
  </Agents>
</FlowReceipt>`;

describe('FlowInspectorComponent', () => {
  let component: FlowInspectorComponent;
  let fixture: ComponentFixture<FlowInspectorComponent>;
  let componentRef: ComponentRef<FlowInspectorComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [FlowInspectorComponent]
    }).compileComponents();

    fixture = TestBed.createComponent(FlowInspectorComponent);
    component = fixture.componentInstance;
    componentRef = fixture.componentRef;
    
    componentRef.setInput('receiptXml', mockXml);
    fixture.detectChanges();
  });

  it('should parse XML and set the first agent as active', () => {
    const receipt = component.parsedReceipt();
    
    expect(receipt).toBeTruthy();
    expect(receipt?.flowId).toBe('flow-999');
    expect(receipt?.task).toBe('Test Task');
    expect(receipt?.agents.length).toBe(2);
    
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
    const emitSpy = vi.spyOn(component.close, 'emit');
    
    // Select the close button specifically
    const closeButton = fixture.nativeElement.querySelector('button');
    closeButton.click();
    
    expect(emitSpy).toHaveBeenCalled();
  });
});