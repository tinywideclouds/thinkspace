import { Injectable, signal, inject } from '@angular/core';
import { TransportService } from '@org/llm-core-transport';
import { ThinkspaceStateService } from './thinkspace-state.service';
import { TimelineStore } from './timeline.store';
import { FlowStore } from './flow.store';
import { CognitiveStore } from './cognitive.store';
import { GraphStore } from './graph.store';
import { UiSettingsService } from './ui-settings.service';
import { LlmFacade, AppEvent, DelegationStrategy, GraphEvent } from '@org/llm-core-facade';
import { ChatItem } from './chat.models';

@Injectable({ providedIn: 'root' })
export class ChatConnectionService {
  private transport = inject(TransportService);
  private workspace = inject(ThinkspaceStateService);
  private timeline = inject(TimelineStore);
  private flow = inject(FlowStore);
  private cognitive = inject(CognitiveStore);
  private graphStore = inject(GraphStore);
  private uiSettings = inject(UiSettingsService);

  public connected = signal<boolean>(false);

  public connect(url: string): void {
    this.transport.connect(url).subscribe({
      next: (protocolBufferEvent) => {
        const domainEvent = LlmFacade.toDomain(protocolBufferEvent);
        if (domainEvent) {
          this.routeIncomingEvent(domainEvent);
        } else {
          console.warn('[DEBUG ChatConnection] LlmFacade.toDomain returned null for:', protocolBufferEvent);
        }
      },
      error: (err) => {
        this.flow.appendSystemLog('ERROR', `Connection error: ${err}`);
        this.connected.set(false);
      },
      complete: () => {
        this.flow.appendSystemLog('INFO', 'Connection closed.');
        this.connected.set(false);
      }
    });
    this.connected.set(true);
  }

  public disconnect(): void {
    this.transport.disconnect();
    this.connected.set(false);
  }

  public clearSession(): void {
    this.graphStore.clear();
    this.flow.manualClearLogs();
    this.cognitive.clearSession();
    this.timeline.setHistoryMetadata('', false, 'main');
  }

  public submitPrompt(text: string): void {
    const spaceId = this.workspace.activeSpaceId();
    const chatId = this.workspace.activeChatId();
    if (!spaceId || !chatId) return;

    this.flow.resetTurn();
    this.cognitive.resetTurn();

    this.transport.send(LlmFacade.createSubmitPrompt(text, spaceId, chatId));
  }

  public loadHistoryPage(): void {
    if (this.timeline.isLoadingHistory() || !this.timeline.hasMore()) return;
    const spaceId = this.workspace.activeSpaceId();
    const chatId = this.workspace.activeChatId();
    if (!spaceId || !chatId) return;

    this.timeline.isLoadingHistory.set(true);
    this.transport.send(LlmFacade.createLoadHistory(
      spaceId, 
      chatId, 
      this.timeline.nextCursor(), 
      this.uiSettings.pagination().size
    ));
  }

  public loadSubGraph(graphId: string): void {
    const spaceId = this.workspace.activeSpaceId();
    const chatId = this.workspace.activeChatId();
    if (!spaceId || !chatId) return;
    
    this.transport.send(LlmFacade.createLoadSubGraph(spaceId, chatId, graphId));
  }

  public submitContextApproval(approved: boolean, paths: string[] = [], indices: number[] = []): void {
    this.flow.resetTurn();
    this.cognitive.pendingContextReview.set(null);
    if (!approved) {
      this.flow.appendSystemLog('INFO', 'Turn aborted by user.');
    }
    this.transport.send(LlmFacade.createApproveContext(approved, paths, indices));
  }

  public selectStrategy(strategy: DelegationStrategy): void {
    this.flow.resetTurn();
    this.cognitive.pendingStrategy.set(false);
    this.transport.send(LlmFacade.createSelectStrategy(strategy));
  }

  public submitReview(accepted: boolean): void {
    this.flow.resetTurn();
    const branch = this.cognitive.pendingReviewBranch();
    this.cognitive.pendingReviewBranch.set(null);
    if (branch) {
      this.transport.send(LlmFacade.createReviewDecision(branch, accepted));
    }
  }

  public submitTransitionDecision(approved: boolean): void {
    this.flow.resetTurn();
    this.cognitive.pendingTransition.set(null);
    this.flow.appendSystemLog('INFO', approved ? 'Transition approved.' : 'Transition aborted.');
    this.transport.send(LlmFacade.createTransitionDecision(approved));
  }

  public forkChat(targetEventId: string): void {
    const spaceId = this.workspace.activeSpaceId();
    const chatId = this.workspace.activeChatId();
    if (!spaceId || !chatId) return;

    this.transport.send(LlmFacade.createForkChat(spaceId, chatId, targetEventId));
  }

  public inlineEdit(targetEventId: string, newText: string): void {
    const spaceId = this.workspace.activeSpaceId();
    const chatId = this.workspace.activeChatId();
    if (!spaceId || !chatId) return;

    this.transport.send(LlmFacade.createInlineEdit(spaceId, chatId, targetEventId, newText));
  }

  public switchBranch(targetBranch: string): void {
    const spaceId = this.workspace.activeSpaceId();
    const chatId = this.workspace.activeChatId();
    if (!spaceId || !chatId) return;

    this.graphStore.clear();
    this.timeline.isLoadingHistory.set(true);

    this.transport.send(LlmFacade.createSwitchBranch(spaceId, chatId, targetBranch));
  }

  private mapGraphEventToChatItem(ev: GraphEvent): ChatItem {
    const base = {
      id: ev.id,
      timestamp: ev.timestamp,
      tombstoned: ev.tombstoned,
      tags: ev.tags,
      metadata: ev.metadata,
      edges: ev.edges,
    };

    switch (ev.type) {
      case 'prompt':
      case 'model':
      case 'system':
        return {
          ...base,
          type: ev.type,
          content: ev.content,
        };

      case 'file_delta':
        return {
          ...base,
          type: 'file_delta',
          content: ev.content,
          path: ev.path,
          previewSnippet: ev.previewSnippet,
          fileRef: ev.fileRef,
          parentId: ev.parentId,
        };

      case 'flow_execution':
        return {
          ...base,
          type: 'flow_execution',
          content: ev.content,
          flowId: ev.flowId,
          taskId: ev.taskId,
          task: ev.task,
          status: ev.status,
          error: ev.error,
          agents: ev.agents,
        };
    }
  }

  private routeIncomingEvent(event: AppEvent): void {
    switch (event.type) {
      case 'sync_history':
        this.graphStore.upsert(event.recentEvents.map(e => this.mapGraphEventToChatItem(e)));
        
        if (event.coordinate) {
          const historyIds = event.recentEvents.map(e => e.id);
          this.graphStore.resetProjection(event.coordinate, historyIds);
          
          if (this.timeline.isActiveView(event.coordinate)) {
            this.timeline.setHistoryMetadata(event.nextCursor, event.hasMore, event.coordinate.projectionId);
            this.cognitive.activeFocus.set(event.focus);
            if (Object.keys(event.digests).length > 0) {
              this.cognitive.activeDigests.set(event.digests);
            }
          }
        }
        this.timeline.isLoadingHistory.set(false);
        break;

      case 'sync_subgraph':
        this.graphStore.upsert(event.events.map(e => this.mapGraphEventToChatItem(e)));
        
        if (event.coordinate) {
          const subGraphIds = event.events.map(e => e.id);
          this.graphStore.resetProjection(event.coordinate, subGraphIds);
        }
        break;

      case 'streamed_graph_event':
        this.graphStore.upsert([this.mapGraphEventToChatItem(event.event)]);
        
        if (event.coordinate) {
          this.graphStore.appendToProjection(event.coordinate, [event.event.id]);
        }
        break;

      case 'preview_context':
        this.cognitive.activeFocus.set(event.context.focus);
        this.cognitive.activeWorkbench.set(event.context.fileContexts);
        if (this.cognitive.autoApproveContext()) {
          const filePaths = event.context.fileContexts.map(f => f.path);
          const historyIndices = event.context.recentHistory.map((_, i) => i);
          this.submitContextApproval(true, filePaths, historyIndices);
        } else {
          this.cognitive.pendingContextReview.set(event.context);
        }
        break;

      case 'chat_stream':
        this.graphStore.nodes.update(map => {
          const next = new Map(map);
          const node = next.get(event.nodeId);
          
          if (node) {
            next.set(event.nodeId, { 
              ...node, 
              content: (node.content || '') + event.text 
            });
          }
          return next;
        });
        break;

      case 'telemetry_event':
        this.flow.processTelemetry(event);
        break;

      case 'request_strategy':
        this.cognitive.pendingStrategy.set(event.active);
        break;
      case 'request_review':
        this.cognitive.pendingReviewBranch.set(event.branch);
        break;
      case 'request_transition':
        this.cognitive.pendingTransition.set({ 
          targetThinkState: event.targetThinkState, 
          reasoning: event.reasoning 
        });
        break;
    }
  }
}