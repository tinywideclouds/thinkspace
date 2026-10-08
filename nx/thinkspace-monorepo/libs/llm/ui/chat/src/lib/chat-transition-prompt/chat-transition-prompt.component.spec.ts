import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ChatTransitionPromptComponent } from './chat-transition-prompt.component';
import { ComponentRef } from '@angular/core';
import { vi, describe, beforeEach, it, expect } from 'vitest';

describe('ChatTransitionPromptComponent', () => {
  let component: ChatTransitionPromptComponent;
  let fixture: ComponentFixture<ChatTransitionPromptComponent>;
  let componentRef: ComponentRef<ChatTransitionPromptComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ChatTransitionPromptComponent]
    }).compileComponents();

    fixture = TestBed.createComponent(ChatTransitionPromptComponent);
    component = fixture.componentInstance;
    componentRef = fixture.componentRef;
    
    componentRef.setInput('targetDomain', 'golang');
    componentRef.setInput('reasoning', 'Need to compile');
    fixture.detectChanges();
  });

  it('should emit true when approved', () => {
    const emitSpy = vi.spyOn(component.transitionDecided, 'emit');
    component.decide(true);
    expect(emitSpy).toHaveBeenCalledWith(true);
  });

  it('should emit false when rejected', () => {
    const emitSpy = vi.spyOn(component.transitionDecided, 'emit');
    component.decide(false);
    expect(emitSpy).toHaveBeenCalledWith(false);
  });
});