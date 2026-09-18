import { Pipe, PipeTransform, inject } from '@angular/core';
import { DomSanitizer, SafeHtml } from '@angular/platform-browser';

@Pipe({ name: 'markdown', standalone: true })
export class MarkdownPipe implements PipeTransform {
  private sanitizer = inject(DomSanitizer);
  
  transform(value: string): SafeHtml {
    if (!value) return '';
    let html = value
      .replace(/```([\s\S]*?)```/g, '<pre style="background: #212529; color: #f8f9fa; padding: 12px; border-radius: 6px; overflow-x: auto; margin: 8px 0; font-family: monospace;"><code>$1</code></pre>')
      .replace(/`([^`]+)`/g, '<code style="background: #e9ecef; padding: 2px 4px; border-radius: 4px; color: #d63384; font-family: monospace;">$1</code>')
      .replace(/### (.*)/g, '<h3 style="margin-top: 16px; margin-bottom: 8px; color: #212529; font-size: 1.2em;">$1</h3>')
      .replace(/#### (.*)/g, '<h4 style="margin-top: 12px; margin-bottom: 8px; color: #495057; font-size: 1.1em;">$1</h4>')
      .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
      .replace(/---/g, '<hr style="border: 0; border-top: 1px solid #dee2e6; margin: 16px 0;" />')
      .replace(/^\* (.*)/gm, '<li style="margin-left: 20px;">$1</li>');

    return this.sanitizer.bypassSecurityTrustHtml(html);
  }
}