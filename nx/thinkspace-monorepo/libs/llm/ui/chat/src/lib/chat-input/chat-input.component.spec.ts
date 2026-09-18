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

  it('should emit sendPrompt with trimmed text on submit', () => {
    const emitSpy = vi.spyOn(component.sendPrompt, 'emit');
    
    component.submit(' Hello World ');

    expect(emitSpy).toHaveBeenCalledWith('Hello World');
  });

  it('should not emit if input is empty', () => {
    const emitSpy = vi.spyOn(component.sendPrompt, 'emit');
    
    component.submit('   ');

    expect(emitSpy).not.toHaveBeenCalled();
  });
});