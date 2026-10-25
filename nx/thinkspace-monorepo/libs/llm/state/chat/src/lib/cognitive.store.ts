import { Injectable, signal, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { ThinkspaceStateService } from './thinkspace-state.service';
import { DomainAttentionFocus, DomainDigestMeta, FileContextMetaData, DomainPreviewContext } from '@org/llm-core-facade';
import { firstValueFrom } from 'rxjs';

@Injectable({ providedIn: 'root' })
export class CognitiveStore {
  private httpClient = inject(HttpClient);
  private workspaceState = inject(ThinkspaceStateService);

  public activeFocus = signal<DomainAttentionFocus | null>(null);
  public activeDigests = signal<Record<string, DomainDigestMeta>>({});
  public activeWorkbench = signal<FileContextMetaData[]>([]);

  public pendingStrategy = signal<boolean>(false);
  public pendingReviewBranch = signal<string | null>(null);
  public pendingContextReview = signal<DomainPreviewContext | null>(null);
  public pendingTransition = signal<{targetDomain: string, reasoning: string} | null>(null);
  public autoApproveContext = signal<boolean>(true);

  public clearSession(): void {
    this.activeFocus.set(null);
    this.activeDigests.set({});
    this.activeWorkbench.set([]);
    this.resetTurn();
  }

  public resetTurn(): void {
    this.pendingStrategy.set(false);
    this.pendingReviewBranch.set(null);
    this.pendingContextReview.set(null);
    this.pendingTransition.set(null);
  }

  public async copyFullContextPreview(previewId: string): Promise<boolean> {
    const spaceId = this.workspaceState.activeSpaceId();
    const chatId = this.workspaceState.activeChatId();
    if (!spaceId || !chatId) return false;

    try {
      const url = `/api/previews/${encodeURIComponent(chatId)}/${encodeURIComponent(previewId)}?space=${encodeURIComponent(spaceId)}`;
      const markdownData = await firstValueFrom(this.httpClient.get(url, { responseType: 'text' }));
      await navigator.clipboard.writeText(markdownData);
      return true;
    } catch {
      return false;
    }
}
  }