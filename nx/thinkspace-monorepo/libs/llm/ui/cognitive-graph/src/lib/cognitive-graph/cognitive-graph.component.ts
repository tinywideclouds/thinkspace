import { Component, inject, output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ChatFlowCardComponent, MarkdownComponent } from '@org/llm-ui-core';
import { CognitiveGraphFacade } from '../cognitive-graph.facade';

@Component({
  selector: 'llm-cognitive-graph',
  standalone: true,
  imports: [CommonModule, MarkdownComponent, ChatFlowCardComponent],
  template: `
    <div class="flex h-full w-full bg-gray-50 overflow-y-auto p-6 font-sans">
      
      <!-- Left Track: Sticky Anchors (Long-Term Memory) -->
      <div class="w-1/3 pr-8 border-r-2 border-dashed border-gray-300 relative">
        <div class="sticky top-0 bg-gray-50 pt-2 pb-4 z-10 mb-4">
          <h3 class="text-xs font-bold uppercase tracking-widest text-orange-600">Core Anchors</h3>
          <p class="text-[10px] text-gray-500">Persistent across horizons</p>
        </div>

        <div class="flex flex-col gap-6">
          @for (node of facade.stickyNodes(); track node.id) {
            <div class="relative p-4 bg-orange-50 border-2 border-orange-200 rounded-xl shadow-sm cursor-pointer transition-all hover:shadow-md hover:-translate-y-0.5 focus:outline-none focus:ring-2 focus:ring-orange-500 block w-full text-left"
                 [class.ring-2]="facade.isNodeInFocus(node.summary)"
                 [class.ring-orange-400]="facade.isNodeInFocus(node.summary)"
                 role="button"
                 tabindex="0"
                 (click)="nodeSelected.emit(node.id)"
                 (keydown.enter)="nodeSelected.emit(node.id)">
              
              <!-- Connection Point -->
              <div class="absolute top-1/2 -right-8 w-8 h-0.5 bg-orange-200"></div>
              <div class="absolute top-1/2 -right-9 w-2 h-2 rounded-full bg-orange-400 -translate-y-1/2"></div>
              
              <div class="flex justify-between items-start mb-2">
                <span class="text-[11px] font-bold text-orange-700">📌 Structural Rule</span>
                <span class="text-[9px] font-mono text-orange-400/70">{{ node.id.split('-')[0] }}</span>
              </div>
              <div class="text-sm text-gray-800">
                <llm-markdown [content]="node.summary"></llm-markdown>
              </div>
            </div>
          }
        </div>
      </div>

      <!-- Right Track: Trajectory & Working Memory -->
      <div class="w-2/3 pl-12 relative">
        
        <!-- Main Spine -->
        <div class="absolute left-0 top-0 bottom-0 w-1 bg-gradient-to-b from-gray-200 via-blue-200 to-blue-400 -translate-x-1/2 rounded-full"></div>

        <div class="sticky top-0 bg-gray-50 pt-2 pb-4 z-10 mb-4 ml-6">
          <h3 class="text-xs font-bold uppercase tracking-widest text-blue-600">Cognitive Trajectory</h3>
          <p class="text-[10px] text-gray-500">Compressed history & active window</p>
        </div>

        <!-- Historical Nodes (Digests) -->
        <div class="flex flex-col gap-8 ml-6 mb-12">
          @for (node of facade.trajectoryNodes(); track node.id) {
            <div class="relative p-5 bg-white border border-gray-200 rounded-xl shadow-sm cursor-pointer transition-all hover:border-blue-300 hover:shadow-md focus:outline-none focus:ring-2 focus:ring-blue-500 block w-full text-left"
                 [class.ring-2]="facade.isNodeInFocus(node.summary)"
                 [class.ring-blue-300]="facade.isNodeInFocus(node.summary)"
                 role="button"
                 tabindex="0"
                 (click)="nodeSelected.emit(node.id)"
                 (keydown.enter)="nodeSelected.emit(node.id)">
              
              <!-- Node Dot -->
              <div class="absolute top-6 -left-7 w-3 h-3 rounded-full border-2 border-white shadow-sm -translate-x-1/2"
                   [ngClass]="facade.isNodeInFocus(node.summary) ? 'bg-blue-400' : 'bg-gray-300'"></div>
              
              <div class="flex justify-between items-center mb-3">
                <span class="text-[10px] font-bold uppercase tracking-wider text-gray-500">📚 Memory Cluster</span>
                <span class="text-[10px] px-2 py-0.5 bg-gray-100 rounded-full text-gray-600 font-mono">{{ node.id.split('-')[0] }}</span>
              </div>
              <div class="text-sm text-gray-700">
                <llm-markdown [content]="node.summary"></llm-markdown>
              </div>
            </div>
          }
        </div>

        <!-- Digestion Horizon Boundary -->
        <div class="relative flex items-center gap-4 my-8 -ml-4">
          <div class="w-8 h-0.5 bg-blue-400 border-t-2 border-dashed border-blue-400"></div>
          <span class="px-3 py-1 bg-blue-50 text-blue-700 text-[10px] font-bold uppercase tracking-widest rounded-full border border-blue-200 shadow-sm">
            Digestion Horizon
          </span>
          <div class="flex-1 h-0.5 border-t-2 border-dashed border-blue-200"></div>
        </div>

        <!-- Working Memory (Raw Chat Items) -->
        <div class="flex flex-col gap-4 ml-6 relative">
          <div class="absolute -left-6 top-0 bottom-0 w-0.5 bg-blue-400/30 border-l-2 border-dashed border-blue-400/50 -translate-x-1/2"></div>
          
          @for (item of facade.activeFeed(); track item.id) {
            <div class="relative">
              <div class="absolute top-1/2 -left-6 w-2 h-2 rounded-full bg-blue-500 shadow-[0_0_8px_rgba(59,130,246,0.6)] -translate-y-1/2 -translate-x-1/2"></div>
              
              @if (item.source === 'flow_card' && item.flowId) {
                <llm-chat-flow-card 
                  [flow]="facade.flowStates().get(item.flowId)" 
                  [summary]="item.content"
                  (inspect)="facade.inspectFlow($event)">
                </llm-chat-flow-card>
              } @else {
                <div class="p-4 rounded-xl border"
                     [ngClass]="item.source === 'user' ? 'bg-blue-50/50 border-blue-100' : 'bg-white border-gray-200'">
                  <strong class="block mb-2 text-[10px] font-bold uppercase tracking-wider"
                          [ngClass]="item.source === 'user' ? 'text-blue-700' : 'text-gray-500'">
                    {{ item.source === 'user' ? 'User Prompt' : 'Manager Output' }}
                  </strong>
                  <div class="text-sm text-gray-800 leading-relaxed">
                    <llm-markdown [content]="item.content"></llm-markdown>
                  </div>
                </div>
              }
            </div>
          }
        </div>

      </div>
    </div>
  `
})
export class CognitiveGraphComponent {
  public facade = inject(CognitiveGraphFacade);
  
  nodeSelected = output<string>();
}