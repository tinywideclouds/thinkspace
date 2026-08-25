import { ComponentFixture, TestBed } from '@angular/core/testing';
import { AppComponent } from './app';
import { ChatStateService } from '@org/llm-state-chat';
import { signal } from '@angular/core';
import { describe, beforeEach, it, expect, vi, Mock } from 'vitest';

describe('AppComponent', () => {
  let component: AppComponent;
  let fixture: ComponentFixture<AppComponent>;
  let mockStateService: {
    connect: Mock;
    submitPrompt: Mock;
    selectStrategy: Mock;
    submitReview: Mock;
    coreChat: ReturnType<typeof signal>;
    pendingStrategy: ReturnType<typeof signal>;
    pendingReviewBranch: ReturnType<typeof signal>;
  };

  beforeEach(async () => {
    mockStateService = {
      connect: vi.fn(),
      submitPrompt: vi.fn(),
      selectStrategy: vi.fn(),
      submitReview: vi.fn(),
      coreChat: signal([]),
      pendingStrategy: signal(false),
      pendingReviewBranch: signal(null),
    };

    await TestBed.configureTestingModule({
      imports: [AppComponent],
      providers: [
        { provide: ChatStateService, useValue: mockStateService }
      ]
    }).compileComponents();

    fixture = TestBed.createComponent(AppComponent);
    component = fixture.componentInstance;
  });

  it('should create the app', () => {
    expect(component).toBeTruthy();
  });

  it('should render standard input when no pending actions exist', () => {
    fixture.detectChanges();
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.querySelector('llm-chat-input')).toBeTruthy();
    expect(compiled.querySelector('llm-chat-strategy-prompt')).toBeFalsy();
    expect(compiled.querySelector('llm-chat-review-prompt')).toBeFalsy();
  });

  it('should render strategy prompt when pendingStrategy is true', () => {
    mockStateService.pendingStrategy.set(true);
    fixture.detectChanges();
    
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.querySelector('llm-chat-strategy-prompt')).toBeTruthy();
    expect(compiled.querySelector('llm-chat-input')).toBeFalsy();
  });

  it('should render review prompt when pendingReviewBranch has a value', () => {
    mockStateService.pendingReviewBranch.set('candidate/circle-123');
    fixture.detectChanges();
    
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.querySelector('llm-chat-review-prompt')).toBeTruthy();
    expect(compiled.querySelector('llm-chat-input')).toBeFalsy();
  });

  it('should connect to the websocket after next render', async () => {
    fixture.detectChanges();
    // Await a microtask to allow afterNextRender callbacks to execute
    await Promise.resolve();
    
    expect(mockStateService.connect).toHaveBeenCalledWith('ws://localhost:8080/ws');
  });
});