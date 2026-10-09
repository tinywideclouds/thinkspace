import { Component, input, computed, HostListener, ViewEncapsulation, inject } from '@angular/core';
import { DomSanitizer } from '@angular/platform-browser';
import { marked, Tokens } from 'marked';
import DOMPurify from 'dompurify';
import hljs from 'highlight.js/lib/core';
import go from 'highlight.js/lib/languages/go';
import typescript from 'highlight.js/lib/languages/typescript';
import json from 'highlight.js/lib/languages/json';
import bash from 'highlight.js/lib/languages/bash';
import xml from 'highlight.js/lib/languages/xml';

// Register only the necessary languages to keep the bundle small
hljs.registerLanguage('go', go);
hljs.registerLanguage('typescript', typescript);
hljs.registerLanguage('json', json);
hljs.registerLanguage('bash', bash);
hljs.registerLanguage('xml', xml);
hljs.registerLanguage('html', xml);

// Configure marked with a custom renderer to inject the Copy button headers
marked.use({
  renderer: {
    code({ text, lang }: Tokens.Code): string {
      const language = lang || 'plaintext';
      const validLanguage = hljs.getLanguage(language) ? language : 'plaintext';
      const highlighted = hljs.highlight(text, { language: validLanguage }).value;
      
      return `
        <div class="code-block-wrapper">
          <div class="code-header">
            <span class="code-lang">${language}</span>
            <button class="copy-btn">Copy</button>
          </div>
          <pre><code class="hljs ${validLanguage}">${highlighted}</code></pre>
        </div>
      `;
    }
  }
});

@Component({
  selector: 'llm-markdown',
  standalone: true,
  encapsulation: ViewEncapsulation.None, // Allows styling the innerHTML output
  templateUrl: './markdown.component.html',
  styleUrl: './markdown.component.scss'
})
export class MarkdownComponent {
  content = input.required<string>();
  private sanitizer = inject(DomSanitizer);

  safeHtml = computed(() => {
    const rawContent = this.content();
    if (!rawContent) return '';

    const html = marked.parse(rawContent) as string;
    
    // Purify the raw HTML to prevent XSS, but preserve the tags/classes injected by marked and highlight.js
    const sanitized = DOMPurify.sanitize(html, {
      ALLOWED_TAGS: ['div', 'span', 'button', 'pre', 'code', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'p', 'ul', 'ol', 'li', 'strong', 'em', 'hr', 'br', 'a', 'blockquote'],
      ALLOWED_ATTR: ['class', 'href', 'target']
    });

    return this.sanitizer.bypassSecurityTrustHtml(sanitized);
  });

  // Intercept native DOM clicks to power the injected Copy buttons
  @HostListener('click', ['$event'])
  onClick(event: Event): void {
    const target = event.target as HTMLElement;
    if (target && target.classList.contains('copy-btn')) {
      const wrapper = target.closest('.code-block-wrapper');
      const pre = wrapper?.querySelector('pre');
      
      if (pre && pre.textContent) {
        navigator.clipboard.writeText(pre.textContent).then(() => {
          const originalText = target.innerText;
          target.innerText = 'Copied!';
          setTimeout(() => {
            target.innerText = originalText;
          }, 2000);
        }).catch(err => {
          console.error('Failed to copy text: ', err);
        });
      }
    }
  }
}