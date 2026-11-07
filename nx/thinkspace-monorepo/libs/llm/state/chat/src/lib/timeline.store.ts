import { Injectable, computed, inject, signal } from '@angular/core';
import { GraphStore } from './graph.store';
import { ChatItem } from './chat.models';
import { BranchTopology } from '@org/llm-core-facade';

@Injectable({ providedIn: 'root' })
export class TimelineStore {
  private graph = inject(GraphStore);

  public isLoadingHistory = signal<boolean>(false);
  public activeBranch = signal<string>('main');
  public branches = signal<Record<string, BranchTopology>>({});
  
  public nextCursor = signal<string>('');
  public hasMore = signal<boolean>(false);

  // Project the timeline from the unified graph cache
  public coreChat = computed<ChatItem[]>(() => {
    const branch = this.activeBranch();
    const allNodes = Array.from(this.graph.nodes().values());
    
    return allNodes
      .filter(node => node.branchId === branch && !node.tombstoned)
      .sort((a, b) => {
        const timeA = a.timestamp.toString();
        const timeB = b.timestamp.toString();
        return timeA.localeCompare(timeB);
      });
  });

  public setHistoryMetadata(
    cursor: string, 
    more: boolean, 
    branch: string, 
    topology: Record<string, BranchTopology>
  ): void {
    this.nextCursor.set(cursor);
    this.hasMore.set(more);
    this.activeBranch.set(branch);
    this.branches.set(topology);
  }
}