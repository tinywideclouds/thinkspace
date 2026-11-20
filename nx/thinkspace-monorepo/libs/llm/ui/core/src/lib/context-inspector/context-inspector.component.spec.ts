import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ContextInspectorComponent } from './context-inspector.component';
import { ContextStage } from '@org/llm-core-facade';
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
      stage: ContextStage.MANAGER,
      managerSystemPrompt: 'Test Manager Rules',
      longTermMemory: ['Sticky rule 1'],
      recentHistory: ['[prompt] user message', '[model] reply', '[catalyst] Build HTTP server'],
      fileContexts: [
        { path: 'src/main.go', level: 'injected', tokenCount: 150 },
        { path: 'src/utils.go', level: 'injected', tokenCount: 20 },
        { path: 'internal/handler/hello.go', level: 'skeleton', tokenCount: 0 },
        { path: 'go.mod', level: 'structural', tokenCount: 0 },
      ],
      focus: { primaryFocus: [], relatedContext: [], dormantLenses: [] },
    });
    fixture.detectChanges();
  });

  it('should split file contexts correctly into injected and mapbook computed arrays', () => {
    expect(component.injectedFiles().length).toBe(2);
    expect(component.mapbookFiles().length).toBe(2);
  });

  it('should render total token calculations accurately using only injected files', () => {
    expect(component.totalFileTokens()).toBe(170);
  });

  it('should render the catalyst as a non-removable Prompt block separate from history checkboxes on Manager stage', () => {
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.textContent).toContain('JIT Context Preview');
    expect(compiled.textContent).toContain('Long-Term Memory:');
    expect(compiled.textContent).toContain('Sticky rule 1');
    expect(compiled.textContent).toContain('Prompt');
    expect(compiled.textContent).toContain('Build HTTP server');
    expect(component.historyEntries().length).toBe(2);
  });

  it('should render mapbook file names inside a collapsed details block by default', () => {
    const compiled = fixture.nativeElement as HTMLElement;
    const detailsEls = Array.from(compiled.querySelectorAll('details'));
    const mapbookDetails = detailsEls.find((d) =>
      d.textContent?.includes('Workspace Mapbook Files (2)'),
    );
    expect(mapbookDetails).toBeTruthy();
    expect(mapbookDetails?.open).toBe(false);
    expect(mapbookDetails?.textContent).toContain('internal/handler/hello.go');
    expect(mapbookDetails?.textContent).toContain('go.mod');
  });

  it('should dynamically update tokens when an injected file is toggled off', () => {
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
      retainedHistoryIndices: [2],
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

  it('should render Sub-Agent Context Preview, Context Digest, and read-only Sub-Agent Tasks on Worker stage', () => {
    componentRef.setInput('context', {
      spaceId: 'golang',
      chatId: 'chat-1',
      previewId: 'prev-worker',
      stage: ContextStage.WORKER,
      managerSystemPrompt: 'Worker Patcher Rules',
      longTermMemory: ['Shared architectural digest'],
      recentHistory: ['[agent-1] Iterative implementation', '[catalyst] Recursive implementation'],
      fileContexts: [
        { path: 'src/main.go', level: 'injected', tokenCount: 150 },
        { path: 'go.mod', level: 'structural', tokenCount: 0 },
      ],
      focus: { primaryFocus: [], relatedContext: [], dormantLenses: [] },
    });
    fixture.detectChanges();

    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.textContent).toContain('Sub-Agent Context Preview');
    expect(compiled.textContent).toContain('Worker Rules & Patcher Schema');
    expect(compiled.textContent).toContain('Context Digest');
    expect(compiled.textContent).toContain('Shared architectural digest');
    expect(compiled.textContent).toContain('Sub-Agent Tasks (2)');
    expect(compiled.textContent).toContain('Iterative implementation');
    expect(compiled.textContent).toContain('Recursive implementation');
    expect(compiled.textContent).toContain('Abort Sub-Agents');
    expect(compiled.textContent).not.toContain('Recent Trajectory');

    const emitSpy = vi.spyOn(component.approve, 'emit');
    const buttons = compiled.querySelectorAll('button');
    buttons[2].click(); // Approve & Run button

    expect(emitSpy).toHaveBeenCalledWith({
      retainedFilePaths: ['src/main.go'],
      retainedHistoryIndices: [0, 1],
    });
  });
});