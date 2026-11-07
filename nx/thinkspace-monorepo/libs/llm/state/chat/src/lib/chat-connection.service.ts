import { Injectable, inject, signal } from '@angular/core';
import { Subscription } from 'rxjs';
import { LlmFacade, AppEvent, DelegationStrategy, GraphEvent } from '@org/llm-core-facade';
import { TransportService } from '@org/llm-core-transport';
import { GraphStore } from './graph.store';
import { TimelineStore } from './timeline.store';
import { CognitiveStore } from './cognitive.store';
import { FlowStore } from './flow.store';
import { ThinkspaceStateService } from './thinkspace-state.service';
import { ChatItem } from './chat.models';

@Injectable({ providedIn: 'root' })
export class ChatConnectionService {
  private graphStore = inject(GraphStore);
  private timelineStore = inject(TimelineStore);
  private cognitiveStore = inject(CognitiveStore);
  private flowStore = inject(FlowStore);
  private workspace = inject(ThinkspaceStateService);
  private transport = inject(TransportService);

  public connected = signal<boolean>(false);
  private connectionSub: Subscription | null = null;

  constructor() {
    this.initConnection();
  }

  public initConnection(wsUrl = 'ws://localhost:8080/ws'): void {
    if (this.connectionSub) {
      this.connectionSub.unsubscribe();
    }

    this.connectionSub = this.transport.connect(wsUrl).subscribe({
      next: (wsEvent) => {
        this.connected.set(true);
        const domainEvent = LlmFacade.toDomain(wsEvent);
        if (domainEvent) {
          this.handleEvent(domainEvent);
        }
      },
      error: () => {
        this.connected.set(false);
      },
      complete: () => {
        this.connected.set(false);
      }
    });
  }

  private mapGraphEventToChatItem(ev: GraphEvent): ChatItem {
    return ev as ChatItem;
  }

  public handleEvent(event: AppEvent): void {
    switch (event.type) {
      case 'sync_history':
        this.graphStore.upsert(event.recentEvents.map(e => this.mapGraphEventToChatItem(e)));
        this.timelineStore.setHistoryMetadata(
          event.nextCursor,
          event.hasMore,
          event.activeBranch,
          event.branches
        );
        this.cognitiveStore.activeDigests.set(event.digests);
        this.cognitiveStore.activeFocus.set(event.focus);
        this.timelineStore.isLoadingHistory.set(false);
        break;

      case 'sync_subgraph':
        this.graphStore.upsert(event.events.map(e => this.mapGraphEventToChatItem(e)));
        break;

      case 'streamed_graph_event':
        this.graphStore.upsert([this.mapGraphEventToChatItem(event.event)]);
        break;
        
      case 'telemetry_event':
        this.flowStore.processTelemetry(event);
        break;

      case 'preview_context':
        this.cognitiveStore.pendingContextReview.set(event.context);
        break;

      case 'request_transition':
        this.cognitiveStore.pendingTransition.set({
          targetThinkState: event.targetThinkState,
          reasoning: event.reasoning
        });
        break;

      case 'request_strategy':
        this.cognitiveStore.pendingStrategy.set(event.active);
        break;

      case 'request_review':
        this.cognitiveStore.pendingReviewBranch.set(event.branch);
        break;
        
      case 'chat_stream':
      case 'agent_stream':
      case 'available_spaces':
        break;
    }
  }

  public submitPrompt(text: string): void {
    const spaceId = this.workspace.activeSpaceId();
    const chatId = this.workspace.activeChatId();
    if (!spaceId || !chatId) return;
    const msg = LlmFacade.createSubmitPrompt(text, spaceId, chatId);
    this.transport.send(msg);
  }

  public loadHistoryPage(): void {
    if (this.timelineStore.isLoadingHistory() || !this.timelineStore.hasMore()) return;
    
    const spaceId = this.workspace.activeSpaceId();
    const chatId = this.workspace.activeChatId();
    if (!spaceId || !chatId) return;

    this.timelineStore.isLoadingHistory.set(true);
    const msg = LlmFacade.createLoadHistory(
      spaceId,
      chatId,
      this.timelineStore.nextCursor(),
      50
    );
    this.transport.send(msg);
  }

  public loadSubGraph(graphId: string): void {
    const spaceId = this.workspace.activeSpaceId();
    const chatId = this.workspace.activeChatId();
    if (!spaceId || !chatId) return;
    
    const msg = LlmFacade.createLoadSubGraph(spaceId, chatId, graphId);
    this.transport.send(msg);
  }

  public selectStrategy(strategy: DelegationStrategy): void {
    const msg = LlmFacade.createSelectStrategy(strategy);
    this.transport.send(msg);
    this.cognitiveStore.pendingStrategy.set(false);
  }

  public submitReview(accepted: boolean): void {
    const branch = this.cognitiveStore.pendingReviewBranch();
    if (!branch) return;
    
    const msg = LlmFacade.createReviewDecision(branch, accepted);
    this.transport.send(msg);
    this.cognitiveStore.pendingReviewBranch.set(null);
  }

  public submitTransitionDecision(approved: boolean): void {
    const msg = LlmFacade.createTransitionDecision(approved);
    this.transport.send(msg);
    this.cognitiveStore.pendingTransition.set(null);
  }

  public submitContextApproval(approved: boolean, retainedFiles: string[] = [], retainedHistory: number[] = []): void {
    const msg = LlmFacade.createApproveContext(approved, retainedFiles, retainedHistory);
    this.transport.send(msg);
    this.cognitiveStore.pendingContextReview.set(null);
  }

  public forkChat(targetEventId: string): void {
    const spaceId = this.workspace.activeSpaceId();
    const chatId = this.workspace.activeChatId();
    if (!spaceId || !chatId) return;
    const msg = LlmFacade.createForkChat(spaceId, chatId, targetEventId);
    this.transport.send(msg);
  }

  public inlineEdit(targetEventId: string, newText: string): void {
    const spaceId = this.workspace.activeSpaceId();
    const chatId = this.workspace.activeChatId();
    if (!spaceId || !chatId) return;
    const msg = LlmFacade.createInlineEdit(spaceId, chatId, targetEventId, newText);
    this.transport.send(msg);
  }

  public switchBranch(targetBranch: string): void {
    const spaceId = this.workspace.activeSpaceId();
    const chatId = this.workspace.activeChatId();
    if (!spaceId || !chatId) return;
    const msg = LlmFacade.createSwitchBranch(spaceId, chatId, targetBranch);
    this.transport.send(msg);
  }

  public clearSession(): void {
    this.graphStore.clear();
    this.flowStore.manualClearLogs();
    this.cognitiveStore.clearSession();
    this.timelineStore.setHistoryMetadata('', false, 'main', {});
  }
}