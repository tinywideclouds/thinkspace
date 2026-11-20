import {
  Component,
  input,
  output,
  computed,
  linkedSignal,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import {
  ContextStage,
  PreviewContext,
  FileContextMetaData,
} from '@org/llm-core-facade';

const CATALYST_PREFIX = '[catalyst] ';
const ROLE_PREFIX_REGEX = /^\[([^\]]+)\]\s*(.*)$/s;

export interface HistoryEntry {
  index: number;
  text: string;
}

export interface SubAgentTaskPreview {
  agentId: string;
  instruction: string;
}

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

  isWorkerStage = computed<boolean>(() => {
    return this.context()?.stage === ContextStage.WORKER;
  });

  prompt = computed<string>(() => {
    if (this.isWorkerStage()) {
      return '';
    }
    const history = this.context()?.recentHistory || [];
    const catalystEntry = history.find((msg) => msg.startsWith(CATALYST_PREFIX));
    return catalystEntry ? catalystEntry.slice(CATALYST_PREFIX.length) : '';
  });

  workerTasks = computed<SubAgentTaskPreview[]>(() => {
    if (!this.isWorkerStage()) {
      return [];
    }
    const history = this.context()?.recentHistory || [];
    return history.map((entry, idx) => {
      if (entry.startsWith(CATALYST_PREFIX)) {
        return {
          agentId: `agent-${idx + 1}`,
          instruction: entry.slice(CATALYST_PREFIX.length),
        };
      }
      const match = entry.match(ROLE_PREFIX_REGEX);
      if (match) {
        return {
          agentId: match[1],
          instruction: match[2],
        };
      }
      return {
        agentId: `agent-${idx + 1}`,
        instruction: entry,
      };
    });
  });

  historyEntries = computed<HistoryEntry[]>(() => {
    if (this.isWorkerStage()) {
      return [];
    }
    const history = this.context()?.recentHistory || [];
    return history
      .map((text, index) => ({ index, text }))
      .filter((entry) => !entry.text.startsWith(CATALYST_PREFIX));
  });

  // Split fileContexts into Injected (Workbench) vs Skeleton/Structural (Mapbook)
  injectedFiles = computed<FileContextMetaData[]>(() => {
    const files = this.context()?.fileContexts || [];
    return files.filter((f) => f.level === 'injected');
  });

  mapbookFiles = computed<FileContextMetaData[]>(() => {
    const files = this.context()?.fileContexts || [];
    return files.filter((f) => f.level === 'skeleton' || f.level === 'structural');
  });

  // Retention state only tracks actionable injected files
  retainedFiles = linkedSignal<Set<string>>(() => {
    return new Set(this.injectedFiles().map((f) => f.path));
  });

  retainedHistory = linkedSignal<Set<number>>(() => {
    return new Set(this.historyEntries().map((entry) => entry.index));
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
    return this.injectedFiles().reduce((sum, file) => {
      return retained.has(file.path) ? sum + file.tokenCount : sum;
    }, 0);
  });

  onApprove() {
    const allHistory = this.context()?.recentHistory || [];

    if (this.isWorkerStage()) {
      this.approve.emit({
        retainedFilePaths: Array.from(this.retainedFiles()),
        retainedHistoryIndices: allHistory.map((_, index) => index),
      });
      return;
    }

    const retained = new Set(this.retainedHistory());
    allHistory.forEach((msg, index) => {
      if (msg.startsWith(CATALYST_PREFIX)) {
        retained.add(index);
      }
    });

    this.approve.emit({
      retainedFilePaths: Array.from(this.retainedFiles()),
      retainedHistoryIndices: Array.from(retained).sort((a, b) => a - b),
    });
  }
}