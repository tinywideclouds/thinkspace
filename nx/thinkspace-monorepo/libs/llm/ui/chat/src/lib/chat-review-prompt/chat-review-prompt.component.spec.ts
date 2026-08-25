import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ChatReviewPromptComponent } from './chat-review-prompt.component';
import { ComponentRef } from '@angular/core';
import { vi, describe, beforeEach, it, expect } from 'vitest';

describe('ChatReviewPromptComponent', () => {
  let component: ChatReviewPromptComponent;
  let fixture: ComponentFixture<ChatReviewPromptComponent>;
  let componentRef: ComponentRef<ChatReviewPromptComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ChatReviewPromptComponent]
    }).compileComponents();

    fixture = TestBed.createComponent(ChatReviewPromptComponent);
    component = fixture.componentInstance;
    componentRef = fixture.componentRef;
    
    componentRef.setInput('branch', 'candidate/test-123');
    fixture.detectChanges();
  });

  it('should emit true when accepted', () => {
    const emitSpy = vi.spyOn(component.reviewDecided, 'emit');
    component.decide(true);
    expect(emitSpy).toHaveBeenCalledWith(true);
  });

  it('should emit false when rejected', () => {
    const emitSpy = vi.spyOn(component.reviewDecided, 'emit');
    component.decide(false);
    expect(emitSpy).toHaveBeenCalledWith(false);
  });
});