import { Component, input, output, computed } from '@angular/core';
import { CommonModule } from '@angular/common';
import { DomainDigestMeta, DomainAttentionFocus } from '@org/llm-core-facade';
import { ChatItem, FlowState } from '@org/llm-state-chat';
import { ChatFlowCardComponent, MarkdownComponent } from '@org/llm-ui-chat';

@Component({
  selector: 'llm-cognitive-timeline',
  standalone: true,
  imports: [CommonModule, MarkdownComponent, ChatFlowCardComponent],
  templateUrl: './cognitive-timeline.component.html',
  styleUrl: './cognitive-timeline.component.scss'
})
export class CognitiveTimelineComponent {
  // Graph Nodes
  digests = input.required<Record<string, DomainDigestMeta>>();
  activeFeed = input.required<ChatItem[]>(); // The working memory that hasn't been digested yet
  flowStates = input.required<Map<string, FlowState>>();
  focus = input<DomainAttentionFocus | null>(null);

  // Interaction
  nodeSelected = output<string>();
  inspectFlow = output<string>();

  // Derived Graph Topology
  stickyNodes = computed(() => {
    return Object.values(this.digests()).filter(d => d.isSticky);
  });

  trajectoryNodes = computed(() => {
    return Object.values(this.digests()).filter(d => !d.isSticky);
  });

  // Helper to determine if a node is currently in the AI's active focus aperture
  isNodeInFocus(summary: string): boolean {
    const activeTags = [
      ...(this.focus()?.primaryFocus || []),
      ...(this.focus()?.relatedContext || [])
    ];
    // In a full implementation, tags would be strongly typed arrays on the DomainDigestMeta.
    // For now, we check if the summary string contains the active #tags.
    return activeTags.some(tag => summary.includes(`#${tag}`));
  }
}