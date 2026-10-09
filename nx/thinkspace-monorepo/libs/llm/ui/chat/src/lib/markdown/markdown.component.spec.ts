import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ComponentRef } from '@angular/core';
import { MarkdownComponent } from './markdown.component';
import { describe, beforeEach, it, expect } from 'vitest';

describe('MarkdownComponent', () => {
  let component: MarkdownComponent;
  let fixture: ComponentFixture<MarkdownComponent>;
  let componentRef: ComponentRef<MarkdownComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [MarkdownComponent]
    }).compileComponents();

    fixture = TestBed.createComponent(MarkdownComponent);
    component = fixture.componentInstance;
    componentRef = fixture.componentRef;
  });

  it('should parse simple markdown paragraphs', () => {
    componentRef.setInput('content', 'Hello **World**');
    fixture.detectChanges();

    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.innerHTML).toContain('<p>Hello <strong>World</strong></p>');
  });

  it('should render a code block with a copy button and syntax highlighting', () => {
    componentRef.setInput('content', '```go\nfunc main() {}\n```');
    fixture.detectChanges();

    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.querySelector('.code-block-wrapper')).toBeTruthy();
    expect(compiled.querySelector('.copy-btn')).toBeTruthy();
    expect(compiled.querySelector('.hljs')).toBeTruthy();
    expect(compiled.textContent).toContain('func main() {}');
  });
});