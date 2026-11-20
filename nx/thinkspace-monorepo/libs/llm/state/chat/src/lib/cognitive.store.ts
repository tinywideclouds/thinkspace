import { Injectable, inject, signal } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { firstValueFrom } from 'rxjs';
import {
  AttentionFocus,
  DigestMeta,
  FileContextMetaData,
  PreviewContext
} from '@org/llm-core-facade';
import { ThinkspaceStateService } from './thinkspace-state.service';

@Injectable({ providedIn: 'root' })
export class CognitiveStore {
  private httpClient = inject(HttpClient);
  private thinkspace = inject(ThinkspaceStateService);

  public activeFocus = signal<AttentionFocus | null>(null);
  public activeDigests = signal<Record<string, DigestMeta>>({});
  public activeWorkbench = signal<FileContextMetaData[]>([]);

  public pendingContextReview = signal<PreviewContext | null>(null);
  public pendingStrategy = signal<boolean>(false);
  public pendingReviewBranch = signal<string | null>(null);
  public pendingTransition = signal<{ targetThinkState: string; reasoning: string } | null>(null);

  public autoApproveContext = signal<boolean>(false);

  public async copyFullContextPreview(previewId: string): Promise<void> {
    const pending = this.pendingContextReview();
    const spaceId = this.thinkspace.activeSpaceId() || pending?.spaceId;
    const chatId = this.thinkspace.activeChatId() || pending?.chatId;

    if (!previewId || !spaceId || !chatId) {
      console.warn('[CognitiveStore] Missing previewId, spaceId, or chatId for preview fetch');
      return;
    }

    const params = new HttpParams().set('space', spaceId);
    const url = `/api/previews/${encodeURIComponent(chatId)}/${encodeURIComponent(previewId)}`;

    try {
      const markdown = await firstValueFrom(
        this.httpClient.get(url, { params, responseType: 'text' })
      );
      await navigator.clipboard.writeText(markdown);
    } catch (error) {
      console.error(`[CognitiveStore] Failed to fetch context preview ${previewId}`, error);
    }
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