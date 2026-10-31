import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ComponentRef } from '@angular/core';
import { ModelMessageBlockComponent } from './model-message-block.component';
import { describe, beforeEach, it, expect, vi } from 'vitest';
import { FileDeltaChatItem } from '@org/llm-state-chat';

describe('ModelMessageBlockComponent', () => {
  let component: ModelMessageBlockComponent;
  let fixture: ComponentFixture<ModelMessageBlockComponent>;
  let componentRef: ComponentRef<ModelMessageBlockComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ModelMessageBlockComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(ModelMessageBlockComponent);
    component = fixture.componentInstance;
    componentRef = fixture.componentRef;

    componentRef.setInput('source', 'model');
    componentRef.setInput('content', 'I am an AI response.');
    componentRef.setInput('tags', ['ui', 'angular']);
    
    await fixture.whenStable();
    fixture.detectChanges();
  });

  it('should create and render content', () => {
    const compiled = fixture.nativeElement as HTMLElement;
    expect(component).toBeTruthy();
    expect(compiled.textContent).toContain('I am an AI response.');
    expect(compiled.textContent).toContain('#ui');
    expect(compiled.textContent).toContain('#angular');
  });

  it('should render attachments if provided', () => {
    const deltaItem: FileDeltaChatItem = {
      id: 'att-1',
      type: 'file_delta',
      path: 'src/main.ts',
      content: 'update',
      previewSnippet: 'export const a = 1;',
      fileRef: 'ref-1',
      timestamp: Temporal.Now.instant()
    };

    componentRef.setInput('attachments', [deltaItem]);
    fixture.detectChanges();

    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.querySelector('llm-file-delta-block')).toBeTruthy();
  });

  it('should emit tagHover on mouseenter and mouseleave', () => {
    const hoverSpy = vi.spyOn(component.tagHover, 'emit');
    const tags = fixture.nativeElement.querySelectorAll('span');
    const uiTag = Array.from(tags).find((t: any) => t.textContent.includes('#ui')) as HTMLElement;
    
    uiTag.dispatchEvent(new MouseEvent('mouseenter'));
    expect(hoverSpy).toHaveBeenCalledWith('ui');
    
    uiTag.dispatchEvent(new MouseEvent('mouseleave'));
    expect(hoverSpy).toHaveBeenCalledWith(null);
  });

  it('should emit tagClick on click and enter keydown', () => {
    const clickSpy = vi.spyOn(component.tagClick, 'emit');
    const tags = fixture.nativeElement.querySelectorAll('span');
    const angularTag = Array.from(tags).find((t: any) => t.textContent.includes('#angular')) as HTMLElement;
    
    angularTag.click();
    expect(clickSpy).toHaveBeenCalledWith('angular');

    const enterEvent = new KeyboardEvent('keydown', { key: 'Enter' });
    angularTag.dispatchEvent(enterEvent);
    expect(clickSpy).toHaveBeenCalledWith('angular');
  });
});