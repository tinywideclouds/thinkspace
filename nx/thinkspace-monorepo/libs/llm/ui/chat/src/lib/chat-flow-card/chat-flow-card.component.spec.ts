import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ChatFlowCardComponent } from './chat-flow-card.component';
import { ComponentRef } from '@angular/core';
import { vi, describe, beforeEach, it, expect } from 'vitest';

describe('ChatFlowCardComponent', () => {
  let component: ChatFlowCardComponent;
  let fixture: ComponentFixture<ChatFlowCardComponent>;
  let componentRef: ComponentRef<ChatFlowCardComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ChatFlowCardComponent]
    }).compileComponents();

    fixture = TestBed.createComponent(ChatFlowCardComponent);
    component = fixture.componentInstance;
    componentRef = fixture.componentRef;
    
    componentRef.setInput('flowId', 'flow-123');
    fixture.detectChanges();
  });

  it('should display the flow ID', () => {
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.textContent).toContain('flow-123');
  });

  it('should emit inspect event with flowId on click', () => {
    const emitSpy = vi.spyOn(component.inspect, 'emit');
    const button = fixture.nativeElement.querySelector('button');
    
    button.click();
    
    expect(emitSpy).toHaveBeenCalledWith('flow-123');
  });
});