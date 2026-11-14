import {
  Component,
  input,
  output,
  computed,
  linkedSignal,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { PreviewContext } from '@org/llm-core-facade';

@Component({
  selector: 'llm-context-inspector',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './context-inspector.component.html',
})
export class ContextInspectorComponent {
  context = input.required<PreviewContext>();

  approve = output<{
    retainedFilePaths: string[];
    retainedHistoryIndices: number[];
  }>();
  reviewCancelled = output<void>();
  copyFull = output<string>();

  retainedFiles = linkedSignal<Set<string>>(() => {
    const files = this.context()?.fileContexts || [];
    return new Set(files.map((f) => f.path));
  });

  retainedHistory = linkedSignal<Set<number>>(() => {
    const history = this.context()?.recentHistory || [];
    return new Set(history.map((_, i) => i));
  });

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

  totalFileTokens = computed(() => {
    const retained = this.retainedFiles();
    const files = this.context()?.fileContexts || [];
    return files.reduce((sum, file) => {
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