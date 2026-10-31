import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ComponentRef, signal } from '@angular/core';
import { UserMessageBlockComponent } from './user-message-block.component';
import { TimelineFacade } from '../timeline.facade';
import { ChatConnectionService } from '@org/llm-state-chat';
import { describe, beforeEach, it, expect, vi } from 'vitest';

describe('UserMessageBlockComponent', () => {
  let component: UserMessageBlockComponent;
  let fixture: ComponentFixture<UserMessageBlockComponent>;
  let componentRef: ComponentRef<UserMessageBlockComponent>;
  let mockFacade: any;
  let mockConnection: any;

  beforeEach(async () => {
    mockFacade = {
      isEditMode: signal(false),
      toggleEditMode: vi.fn()
    };
    
    mockConnection = {
      inlineEdit: vi.fn(),
      forkChat: vi.fn()
    };

    await TestBed.configureTestingModule({
      imports: [UserMessageBlockComponent],
      providers: [
        { provide: TimelineFacade, useValue: mockFacade },
        { provide: ChatConnectionService, useValue: mockConnection }
      ]
    }).compileComponents();

    fixture = TestBed.createComponent(UserMessageBlockComponent);
    component = fixture.componentInstance;
    componentRef = fixture.componentRef;

    componentRef.setInput('id', 'msg-123');
    componentRef.setInput('content', 'Hello World');
    
    await fixture.whenStable();
    fixture.detectChanges();
  });

  it('should render the provided content', () => {
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.textContent).toContain('Hello World');
  });

  it('should not display edit actions when edit mode is false', () => {
    const buttons = fixture.nativeElement.querySelectorAll('button');
    expect(buttons.length).toBe(0);
  });

  it('should display edit and fork actions when edit mode is active', () => {
    mockFacade.isEditMode.set(true);
    fixture.detectChanges();
    
    const buttons = fixture.nativeElement.querySelectorAll('button');
    expect(buttons.length).toBe(2);
    expect(buttons[0].textContent).toContain('Edit');
    expect(buttons[1].textContent).toContain('Fork');
  });

  it('should swap to inline composer when Edit is clicked', () => {
    mockFacade.isEditMode.set(true);
    fixture.detectChanges();
    
    const editBtn = fixture.nativeElement.querySelectorAll('button')[0];
    editBtn.click();
    fixture.detectChanges();

    expect(component.isEditing()).toBe(true);
    const composer = fixture.nativeElement.querySelector('llm-inline-edit-composer');
    expect(composer).toBeTruthy();
  });

  it('should execute inline edit and exit mode on save', () => {
    component.isEditing.set(true);
    fixture.detectChanges();

    component.onSave('Updated prompt');
    
    expect(mockConnection.inlineEdit).toHaveBeenCalledWith('msg-123', 'Updated prompt');
    expect(component.isEditing()).toBe(false);
    expect(mockFacade.toggleEditMode).toHaveBeenCalled();
  });
});