import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ContextInspectorComponent } from './context-inspector.component';
import { ComponentRef } from '@angular/core';
import { vi, describe, beforeEach, it, expect } from 'vitest';

describe('ContextInspectorComponent', () => {
  let component: ContextInspectorComponent;
  let fixture: ComponentFixture<ContextInspectorComponent>;
  let componentRef: ComponentRef<ContextInspectorComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ContextInspectorComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(ContextInspectorComponent);
    component = fixture.componentInstance;
    componentRef = fixture.componentRef;

    componentRef.setInput('context', {
      spaceId: 'golang',
      chatId: 'chat-1',
      previewId: 'prev-123',
      managerSystemPrompt: 'Test Manager Rules',
      stickyDigests: ['Sticky rule 1'],
      recentHistory: ['[prompt] user message', '[model] reply', '[catalyst] Build HTTP server'],
      fileContexts: [
        { path: 'src/main.go', level: 'skeleton', tokenCount: 150 },
        { path: 'src/utils.go', level: 'structural', tokenCount: 20 },
      ],
    });
    fixture.detectChanges();
  });

  it('should render total token calculations accurately', () => {
    expect(component.totalFileTokens()).toBe(170);
  });

  it('should render the catalyst as a non-removable Prompt block separate from history checkboxes', () => {
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.textContent).toContain('Prompt');
    expect(compiled.textContent).toContain('Build HTTP server');
    expect(component.historyEntries().length).toBe(2);
  });

  it('should dynamically update tokens when a file is toggled off', () => {
    component.toggleFile('src/utils.go');
    fixture.detectChanges();
    expect(component.totalFileTokens()).toBe(150);
  });

  it('should always retain the catalyst index when approving even if all history items are unchecked', () => {
    const emitSpy = vi.spyOn(component.approve, 'emit');

    component.toggleHistory(0);
    component.toggleHistory(1);
    fixture.detectChanges();

    const buttons = fixture.nativeElement.querySelectorAll('button');
    buttons[2].click(); // Approve & Send button

    expect(emitSpy).toHaveBeenCalledWith({
      retainedFilePaths: ['src/main.go', 'src/utils.go'],
      retainedHistoryIndices: [2], // Catalyst index 2 is always retained
    });
  });

  it('should emit reviewCancelled when Cancel Prompt is clicked', () => {
    const emitSpy = vi.spyOn(component.reviewCancelled, 'emit');
    const buttons = fixture.nativeElement.querySelectorAll('button');
    buttons[1].click(); // Cancel Prompt button
    expect(emitSpy).toHaveBeenCalled();
  });

  it('should emit copyFull with the previewId when Copy button is clicked', () => {
    const emitSpy = vi.spyOn(component.copyFull, 'emit');
    const buttons = fixture.nativeElement.querySelectorAll('button');
    buttons[0].click(); // Copy Full Payload button
    expect(emitSpy).toHaveBeenCalledWith('prev-123');
  });
});