import { Component, input, output, computed, signal, effect } from '@angular/core';
import { CommonModule } from '@angular/common';

interface ParsedAgent {
  agentId: string;
  passed: boolean;
  instruction: string;
  rawPayload: string;
  verificationTrace: string;
  stateDelta: string;
}

interface ParsedReceipt {
  flowId: string;
  taskId: string;
  task: string;
  summary: string;
  agents: ParsedAgent[];
}

@Component({
  selector: 'llm-flow-inspector',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './flow-inspector.component.html'
})
export class FlowInspectorComponent {
  receiptXml = input.required<string>();
  close = output<void>();

  parsedReceipt = signal<ParsedReceipt | null>(null);
  selectedAgentId = signal<string | null>(null);

  constructor() {
    // Parse the XML whenever the input string changes
    effect(() => {
      const xml = this.receiptXml();
      if (!xml) return;

      const parser = new DOMParser();
      const doc = parser.parseFromString(xml, 'application/xml');

      const flowReceiptNode = doc.querySelector('FlowReceipt');
      if (!flowReceiptNode) return;

      const agents: ParsedAgent[] = [];
      const agentNodes = doc.querySelectorAll('Agent');
      agentNodes.forEach(node => {
        agents.push({
          agentId: node.getAttribute('AgentID') || '',
          passed: node.getAttribute('Passed') === 'true',
          instruction: node.querySelector('Instruction')?.textContent || '',
          rawPayload: node.querySelector('RawPayload')?.textContent || '',
          verificationTrace: node.querySelector('VerificationTrace')?.textContent || '',
          stateDelta: node.querySelector('StateDelta')?.textContent || ''
        });
      });

      const receipt: ParsedReceipt = {
        flowId: flowReceiptNode.getAttribute('FlowID') || '',
        taskId: flowReceiptNode.getAttribute('TaskID') || '',
        task: doc.querySelector('Task')?.textContent || '',
        summary: doc.querySelector('Summary')?.textContent || '',
        agents
      };

      this.parsedReceipt.set(receipt);
      if (agents.length > 0) {
        this.selectedAgentId.set(agents[0].agentId);
      }
    }, { allowSignalWrites: true });
  }

  activeAgent = computed(() => {
    const receipt = this.parsedReceipt();
    const id = this.selectedAgentId();
    if (!receipt || !id) return null;
    return receipt.agents.find(a => a.agentId === id) || null;
  });
}