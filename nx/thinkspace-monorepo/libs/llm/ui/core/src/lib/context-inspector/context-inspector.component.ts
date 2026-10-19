import {
  Component,
  input,
  output,
  signal,
  computed,
  effect,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { DomainPreviewContext } from '@org/llm-core-facade';

@Component({
  selector: 'llm-context-inspector',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div
      style="background: #f8f9fa; border: 1px solid #ced4da; border-radius: 8px; margin-top: 16px; overflow: hidden; display: flex; flex-direction: column;"
    >
      <!-- Header & Actions -->
      <div
        style="background: #e9ecef; padding: 12px 16px; border-bottom: 1px solid #ced4da; display: flex; justify-content: space-between; align-items: center;"
      >
        <div style="display: flex; align-items: center; gap: 8px;">
          <strong style="color: #212529; font-size: 14px;"
            >🧠 JIT Context Preview</strong
          >
          <span
            style="font-size: 11px; background: #fff; padding: 2px 6px; border-radius: 12px; border: 1px solid #dee2e6; color: #495057;"
          >
            ~{{ totalFileTokens() }} File Tokens
          </span>
        </div>
        <div style="display: flex; gap: 8px;">
          <button
            (click)="copyFull.emit(context().previewId!)"
            style="background: #fff; border: 1px solid #adb5bd; padding: 6px 12px; border-radius: 4px; cursor: pointer; font-size: 12px; font-weight: bold; color: #495057;"
          >
            📋 Copy Full Payload
          </button>
          <button
            (click)="reviewCancelled.emit()"
            style="background: #fff; border: 1px solid #ffa8a8; padding: 6px 12px; border-radius: 4px; cursor: pointer; font-size: 12px; font-weight: bold; color: #e03131;"
          >
            Abort
          </button>
          <button
            (click)="onApprove()"
            style="background: #339af0; border: 1px solid #228be6; padding: 6px 12px; border-radius: 4px; cursor: pointer; font-size: 12px; font-weight: bold; color: #fff;"
          >
            Approve & Send 🚀
          </button>
        </div>
      </div>

      <div
        style="padding: 16px; display: flex; flex-direction: column; gap: 16px; max-height: 400px; overflow-y: auto;"
      >
        <!-- Manager System Prompt (Collapsible) -->
        <details
          style="background: #fff; border: 1px solid #dee2e6; border-radius: 4px; padding: 8px;"
        >
          <summary
            style="cursor: pointer; font-size: 12px; font-weight: bold; color: #495057; text-transform: uppercase;"
          >
            Manager Rules & Persona
          </summary>
          <pre
            style="margin-top: 8px; font-size: 11px; color: #868e96; white-space: pre-wrap; font-family: monospace;"
            >{{ context().managerSystemPrompt }}</pre>
        </details>

        <!-- Temporal Memory -->
        <div style="display: flex; flex-direction: column; gap: 8px;">
          <div
            style="font-size: 12px; font-weight: bold; color: #495057; text-transform: uppercase; border-bottom: 1px solid #dee2e6; padding-bottom: 4px;"
          >
            Temporal Memory
          </div>

          @if (context().stickyDigests.length > 0) {
            <div style="font-size: 12px; font-weight: bold; color: #d9480f;">
              Sticky Context:
            </div>
            <ul
              style="margin: 0; padding-left: 20px; font-size: 12px; color: #495057;"
            >
              @for (digest of context().stickyDigests; track digest) {
                <li>{{ digest }}</li>
              }
            </ul>
          }

          @if (context().recentHistory.length > 0) {
            <div
              style="font-size: 12px; font-weight: bold; color: #1971c2; margin-top: 4px;"
            >
              Recent Trajectory (Sliding Window):
              <span
                style="font-weight: normal; font-size: 11px; color: #868e96; margin-left: 4px;"
              >
                (Uncheck to omit from working memory)
              </span>
            </div>
            <div
              style="background: #f1f3f5; border-radius: 4px; padding: 8px; display: flex; flex-direction: column; gap: 4px;"
            >
              @for (msg of context().recentHistory; track $index) {
                <label
                  style="display: flex; gap: 8px; align-items: flex-start; cursor: pointer; transition: opacity 0.2s;"
                  [style.opacity]="retainedHistory().has($index) ? '1' : '0.4'"
                >
                  <input
                    type="checkbox"
                    [checked]="retainedHistory().has($index)"
                    (change)="toggleHistory($index)"
                    style="margin-top: 4px;"
                  />
                  <div
                    style="font-family: monospace; font-size: 11px; color: #495057; white-space: pre-wrap;"
                  >
                    {{ msg }}
                  </div>
                </label>
              }
            </div>
          } @else {
            <div style="font-size: 11px; color: #adb5bd; font-style: italic;">
              No recent chat history.
            </div>
          }
        </div>

        <!-- Spatial Reality (Files) -->
        <div style="display: flex; flex-direction: column; gap: 8px;">
          <div
            style="font-size: 12px; font-weight: bold; color: #495057; text-transform: uppercase; border-bottom: 1px solid #dee2e6; padding-bottom: 4px;"
          >
            Spatial Reality (Injected Files)
          </div>

          @if (context().fileContexts.length === 0) {
            <div style="font-size: 11px; color: #adb5bd; font-style: italic;">
              No files requested for this turn.
            </div>
          } @else {
            <div style="display: flex; flex-direction: column; gap: 4px;">
              @for (file of context().fileContexts; track file.path) {
                <label
                  style="display: flex; justify-content: space-between; align-items: center; background: #fff; border: 1px solid #dee2e6; padding: 6px 12px; border-radius: 4px; cursor: pointer; transition: opacity 0.2s;"
                  [style.opacity]="retainedFiles().has(file.path) ? '1' : '0.4'"
                >
                  <div style="display: flex; gap: 8px; align-items: center;">
                    <input
                      type="checkbox"
                      [checked]="retainedFiles().has(file.path)"
                      (change)="toggleFile(file.path)"
                    />
                    <code style="font-size: 12px; color: #212529;">{{
                      file.path
                    }}</code>
                  </div>
                  <div style="display: flex; align-items: center; gap: 8px;">
                    <span
                      [style.background]="
                        file.level === 'structural'
                          ? '#e7f5ff'
                          : file.level === 'skeleton'
                            ? '#fff3cd'
                            : '#ebfbee'
                      "
                      [style.color]="
                        file.level === 'structural'
                          ? '#1971c2'
                          : file.level === 'skeleton'
                            ? '#b19700'
                            : '#2b8a3e'
                      "
                      style="font-size: 10px; font-weight: bold; padding: 2px 6px; border-radius: 12px; text-transform: uppercase;"
                    >
                      {{ file.level }}
                    </span>
                    <span
                      style="font-size: 11px; color: #868e96; font-family: monospace; width: 60px; text-align: right;"
                    >
                      ~{{ file.tokenCount }} t
                    </span>
                  </div>
                </label>
              }
            </div>
          }
        </div>
      </div>
    </div>
  `,
})
export class ContextInspectorComponent {
  context = input.required<DomainPreviewContext & { previewId?: string }>();

  approve = output<{
    retainedFilePaths: string[];
    retainedHistoryIndices: number[];
  }>();
  reviewCancelled = output<void>();
  copyFull = output<string>();

  // We track history items by array index rather than UUID.
  // Because the context window is assembled Just-In-Time (JIT), the frontend and backend
  // don't need to cross-reference permanent database IDs to prune the active prompt.
  // When approved, the backend simply slices its working memory array using these structural indices.
  retainedFiles = signal<Set<string>>(new Set());
  retainedHistory = signal<Set<number>>(new Set());

  constructor() {
    effect(
      () => {
        const ctx = this.context();
        if (ctx) {
          // Default to everything fully selected when the component opens
          this.retainedFiles.set(
            new Set(ctx.fileContexts?.map((f) => f.path) || []),
          );
          this.retainedHistory.set(
            new Set(ctx.recentHistory?.map((_, i) => i) || []),
          );
        }
      },
      { allowSignalWrites: true },
    );
  }

  toggleFile(path: string): void {
    const current = new Set(this.retainedFiles());
    if (current.has(path)) {
      current.delete(path);
    } else {
      current.add(path);
    }
    this.retainedFiles.set(current);
  }

  toggleHistory(index: number): void {
    const current = new Set(this.retainedHistory());
    if (current.has(index)) {
      current.delete(index);
    } else {
      current.add(index);
    }
    this.retainedHistory.set(current);
  }

  // Recalculates dynamically as checkboxes are toggled
  totalFileTokens = computed(() => {
    const retained = this.retainedFiles();
    return this.context().fileContexts.reduce((sum, file) => {
      return retained.has(file.path) ? sum + file.tokenCount : sum;
    }, 0);
  });

  onApprove() {
    this.approve.emit({
      retainedFilePaths: Array.from(this.retainedFiles()),
      retainedHistoryIndices: Array.from(this.retainedHistory()),
    });
  }
}
