import { Injectable, signal } from '@angular/core';
import { 
  AttentionFocus, 
  DigestMeta, 
  FileContextMetaData, 
  PreviewContext 
} from '@org/llm-core-facade';

@Injectable({ providedIn: 'root' })
export class CognitiveStore {
  public activeFocus = signal<AttentionFocus | null>(null);
  public activeDigests = signal<Record<string, DigestMeta>>({});
  public activeWorkbench = signal<FileContextMetaData[]>([]);

  public pendingContextReview = signal<PreviewContext | null>(null);
  public pendingStrategy = signal<boolean>(false);
  public pendingReviewBranch = signal<string | null>(null);
  public pendingTransition = signal<{ targetThinkState: string; reasoning: string } | null>(null);

  public autoApproveContext = signal<boolean>(false);

  public copyFullContextPreview(previewId: string): void {
    console.log(`[CognitiveStore] Copying full context preview for ${previewId}`);
    navigator.clipboard.writeText(`Mock Full Context Payload for ${previewId}`);
  }

  public clearSession(): void {
    this.activeFocus.set(null);
    this.activeDigests.set({});
    this.activeWorkbench.set([]);
    this.resetTurn();
  }

  public resetTurn(): void {
    this.pendingContextReview.set(null);
    this.pendingStrategy.set(false);
    this.pendingReviewBranch.set(null);
    this.pendingTransition.set(null);
  }
}