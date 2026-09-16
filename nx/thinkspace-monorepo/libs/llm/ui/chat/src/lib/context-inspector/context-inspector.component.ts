import { Component, input, output, signal, computed } from '@angular/core';
import { CommonModule } from '@angular/common';
import { DomainPreviewContext } from '@org/llm-core-facade';

@Component({
  selector: 'llm-context-inspector',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div style="background: #f8f9fa; border: 1px solid #ced4da; border-radius: 8px; margin-top: 16px; overflow: hidden; display: flex; flex-direction: column;">
      
      <!-- Header & Actions -->
      <div style="background: #e9ecef; padding: 12px 16px; border-bottom: 1px solid #ced4da; display: flex; justify-content: space-between; align-items: center;">
        <div style="display: flex; align-items: center; gap: 8px;">
          <strong style="color: #212529; font-size: 14px;">🧠 JIT Context Preview</strong>
          <span style="font-size: 11px; background: #fff; padding: 2px 6px; border-radius: 12px; border: 1px solid #dee2e6; color: #495057;">
            ~{{ totalFileTokens() }} File Tokens
          </span>
        </div>
        <div style="display: flex; gap: 8px;">
          <button 
            (click)="copyFull.emit(context().previewId!)"
            style="background: #fff; border: 1px solid #adb5bd; padding: 6px 12px; border-radius: 4px; cursor: pointer; font-size: 12px; font-weight: bold; color: #495057;">
            📋 Copy Full Payload
          </button>
          <button 
            (click)="reviewCancelled.emit()"
            style="background: #fff; border: 1px solid #ffa8a8; padding: 6px 12px; border-radius: 4px; cursor: pointer; font-size: 12px; font-weight: bold; color: #e03131;">
            Abort
          </button>
          <button 
            (click)="approve.emit()"
            style="background: #339af0; border: 1px solid #228be6; padding: 6px 12px; border-radius: 4px; cursor: pointer; font-size: 12px; font-weight: bold; color: #fff;">
            Approve & Send 🚀
          </button>
        </div>
      </div>

      <div style="padding: 16px; display: flex; flex-direction: column; gap: 16px; max-height: 400px; overflow-y: auto;">
        
        <!-- Manager System Prompt (Collapsible) -->
        <details style="background: #fff; border: 1px solid #dee2e6; border-radius: 4px; padding: 8px;">
          <summary style="cursor: pointer; font-size: 12px; font-weight: bold; color: #495057; text-transform: uppercase;">
            Manager Rules & Persona
          </summary>
          <pre style="margin-top: 8px; font-size: 11px; color: #868e96; white-space: pre-wrap; font-family: monospace;">{{ context().managerSystemPrompt }}</pre>
        </details>

        <!-- Temporal Memory -->
        <div style="display: flex; flex-direction: column; gap: 8px;">
          <div style="font-size: 12px; font-weight: bold; color: #495057; text-transform: uppercase; border-bottom: 1px solid #dee2e6; padding-bottom: 4px;">
            Temporal Memory
          </div>
          
          @if (context().stickyDigests.length > 0) {
            <div style="font-size: 12px; font-weight: bold; color: #d9480f;">Sticky Context:</div>
            <ul style="margin: 0; padding-left: 20px; font-size: 12px; color: #495057;">
              @for (digest of context().stickyDigests; track digest) {
                <li>{{ digest }}</li>
              }
            </ul>
          }

          @if (context().recentHistory.length > 0) {
            <div style="font-size: 12px; font-weight: bold; color: #1971c2; margin-top: 4px;">Recent Trajectory (Sliding Window):</div>
            <div style="background: #f1f3f5; border-radius: 4px; padding: 8px; display: flex; flex-direction: column; gap: 4px;">
              @for (msg of context().recentHistory; track $index) {
                <div style="font-family: monospace; font-size: 11px; color: #495057; white-space: pre-wrap;">{{ msg }}</div>
              }
            </div>
          } @else {
            <div style="font-size: 11px; color: #adb5bd; font-style: italic;">No recent chat history.</div>
          }
        </div>

        <!-- Spatial Reality (Files) -->
        <div style="display: flex; flex-direction: column; gap: 8px;">
          <div style="font-size: 12px; font-weight: bold; color: #495057; text-transform: uppercase; border-bottom: 1px solid #dee2e6; padding-bottom: 4px;">
            Spatial Reality (Injected Files)
          </div>
          
          @if (context().fileContexts.length === 0) {
            <div style="font-size: 11px; color: #adb5bd; font-style: italic;">No files requested for this turn.</div>
          } @else {
            <div style="display: flex; flex-direction: column; gap: 4px;">
              @for (file of context().fileContexts; track file.path) {
                <div style="display: flex; justify-content: space-between; align-items: center; background: #fff; border: 1px solid #dee2e6; padding: 6px 12px; border-radius: 4px;">
                  <code style="font-size: 12px; color: #212529;">{{ file.path }}</code>
                  <div style="display: flex; align-items: center; gap: 8px;">
                    <span 
                      [style.background]="file.level === 'structural' ? '#e7f5ff' : (file.level === 'skeleton' ? '#fff3cd' : '#ebfbee')"
                      [style.color]="file.level === 'structural' ? '#1971c2' : (file.level === 'skeleton' ? '#b19700' : '#2b8a3e')"
                      style="font-size: 10px; font-weight: bold; padding: 2px 6px; border-radius: 12px; text-transform: uppercase;">
                      {{ file.level }}
                    </span>
                    <span style="font-size: 11px; color: #868e96; font-family: monospace; width: 60px; text-align: right;">
                      ~{{ file.tokenCount }} t
                    </span>
                  </div>
                </div>
              }
            </div>
          }
        </div>

      </div>
    </div>
  `
})
export class ContextInspectorComponent {
  context = input.required<DomainPreviewContext & { previewId?: string }>();
  
  approve = output<void>();
  reviewCancelled = output<void>();
  copyFull = output<string>();

  totalFileTokens = computed(() => {
    return this.context().fileContexts.reduce((sum, file) => sum + file.tokenCount, 0);
  });
}