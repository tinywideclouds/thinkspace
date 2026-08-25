import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ChatInputComponent } from './chat-input.component';
import { vi, describe, beforeEach, it, expect } from 'vitest';

describe('ChatInputComponent', () => {
  let component: ChatInputComponent;
  let fixture: ComponentFixture<ChatInputComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ChatInputComponent]
    }).compileComponents();

    fixture = TestBed.createComponent(ChatInputComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should emit sendPrompt and clear input on submit', () => {
    const emitSpy = vi.spyOn(component.sendPrompt, 'emit');
    
    component.promptText.set(' Hello World ');
    component.submit();

    expect(emitSpy).toHaveBeenCalledWith('Hello World');
    expect(component.promptText()).toBe('');
  });

  it('should not emit if input is empty', () => {
    const emitSpy = vi.spyOn(component.sendPrompt, 'emit');
    
    component.promptText.set('   ');
    component.submit();

    expect(emitSpy).not.toHaveBeenCalled();
  });
});