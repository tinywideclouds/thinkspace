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
  templateUrl: './context-inspector.component.html',
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
    effect(() => {
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
    });
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