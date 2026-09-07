import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ComponentRef } from '@angular/core';
import { ChatHeaderComponent } from './chat-header.component';
import { vi, describe, beforeEach, it, expect } from 'vitest';

describe('ChatHeaderComponent', () => {
  let component: ChatHeaderComponent;
  let fixture: ComponentFixture<ChatHeaderComponent>;
  let componentReference: ComponentRef<ChatHeaderComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ChatHeaderComponent]
    }).compileComponents();

    fixture = TestBed.createComponent(ChatHeaderComponent);
    component = fixture.componentInstance;
    componentReference = fixture.componentRef;
    
    componentReference.setInput('spaces', [
      { id: 'golang', name: 'Go Developer', isConfigured: true }
    ]);
    componentReference.setInput('activeSpaceId', 'golang');
    
    componentReference.setInput('chats', [
      { id: 'chat-1', name: 'First Chat', createdAt: '2026-09-07T00:00:00Z' },
      { id: 'chat-2', name: 'Second Chat', createdAt: '2026-09-07T01:00:00Z' }
    ]);
    componentReference.setInput('activeChatId', 'chat-1');

    fixture.detectChanges();
  });

  it('should emit spaceSelected when a new space is chosen', () => {
    const emitSpy = vi.spyOn(component.spaceSelected, 'emit');
    const selectElement = fixture.nativeElement.querySelector('#spaceSelect') as HTMLSelectElement;
    
    selectElement.value = 'golang';
    selectElement.dispatchEvent(new Event('change'));
    
    expect(emitSpy).toHaveBeenCalledWith('golang');
  });

  it('should emit chatSelected when a new chat is chosen', () => {
    const emitSpy = vi.spyOn(component.chatSelected, 'emit');
    const selectElement = fixture.nativeElement.querySelector('#chatSelect') as HTMLSelectElement;
    
    selectElement.value = 'chat-2';
    selectElement.dispatchEvent(new Event('change'));
    
    expect(emitSpy).toHaveBeenCalledWith('chat-2');
  });

  it('should emit newChatRequested when the new button is clicked', () => {
    const emitSpy = vi.spyOn(component.newChatRequested, 'emit');
    const newButton = fixture.nativeElement.querySelector('button') as HTMLButtonElement;
    
    newButton.click();
    
    expect(emitSpy).toHaveBeenCalled();
  });
});