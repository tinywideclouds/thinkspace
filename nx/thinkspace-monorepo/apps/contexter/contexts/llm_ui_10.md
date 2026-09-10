

### `libs\llm\ui\chat\eslint.config.mjs`
```
import nx from '@nx/eslint-plugin';
import baseConfig from '../../../../eslint.config.mjs';

export default [
  ...nx.configs['flat/angular'],
  ...nx.configs['flat/angular-template'],
  ...baseConfig,
  {
    files: ['**/*.ts'],
    rules: {
      '@angular-eslint/directive-selector': [
        'error',
        {
          type: 'attribute',
          prefix: 'lib',
          style: 'camelCase',
        },
      ],
      '@angular-eslint/component-selector': [
        'error',
        {
          type: 'element',
          prefix: 'lib',
          style: 'kebab-case',
        },
      ],
    },
  },
  {
    files: ['**/*.html'],
    // Override or add rules here
    rules: {},
  },
];

```


### `libs\llm\ui\chat\project.json`
```
{
  "name": "llm-ui-chat",
  "$schema": "../../../../node_modules/nx/schemas/project-schema.json",
  "sourceRoot": "libs/llm/ui/chat/src",
  "prefix": "llm",
  "projectType": "library",
  "tags": ["scope:llm"],
  "targets": {
    "lint": {
      "executor": "@nx/eslint:lint"
    }
  }
}

```


### `libs\llm\ui\chat\README.md`
```
# llm-ui-chat

This library provides the modern Angular (v22+) presentation components for the ThinkSpace chat interface. 

## Responsibilities
* Render the chat feeds, logs, and interactive CLI prompts.
* Provide strict "dumb" components utilizing modern Signal `input()` and `output()` APIs.
* Maintain strict separation of concerns with isolated HTML and SCSS files.

## Components
* `ChatFeedComponent`: Renders the linear message feed.
* `ChatInputComponent`: Standard user prompt entry.
* `ChatStrategyPromptComponent`: Interactive buttons for delegation strategy (Refine, Review, etc.).
* `ChatReviewPromptComponent`: Interactive approval for generated candidate branches.

## Rules
* **MUST NOT** hold business logic, connect to WebSockets, or inject State services directly.
* Data flows *in* via `[inputs]`, actions flow *out* via `(outputs)`.
```


### `libs\llm\ui\chat\tsconfig.json`
```
{
  "extends": "../../../../tsconfig.base.json",
  "compilerOptions": {
    "isolatedModules": true,
    "target": "es2022",
    "noImplicitOverride": true,
    "noPropertyAccessFromIndexSignature": true,
    "noImplicitReturns": true,
    "noFallthroughCasesInSwitch": true,
    "emitDecoratorMetadata": false,
    "module": "preserve"
  },
  "angularCompilerOptions": {
    "enableI18nLegacyMessageIdFormat": false,
    "strictInjectionParameters": true,
    "strictInputAccessModifiers": true,
    "strictTemplates": true
  },
  "files": [],
  "include": [],
  "references": [
    {
      "path": "./tsconfig.lib.json"
    },
    {
      "path": "./tsconfig.spec.json"
    }
  ]
}

```


### `libs\llm\ui\chat\tsconfig.lib.json`
```
{
  "extends": "./tsconfig.json",
  "compilerOptions": {
    "outDir": "../../../../dist/out-tsc",
    "declaration": true,
    "declarationMap": true,
    "inlineSources": true,
    "types": []
  },
  "include": ["src/**/*.ts"],
  "exclude": [
    "src/**/*.spec.ts",
    "src/**/*.test.ts",
    "vite.config.ts",
    "vite.config.mts",
    "vitest.config.ts",
    "vitest.config.mts",
    "src/**/*.test.tsx",
    "src/**/*.spec.tsx",
    "src/**/*.test.js",
    "src/**/*.spec.js",
    "src/**/*.test.jsx",
    "src/**/*.spec.jsx",
    "src/test-setup.ts"
  ]
}

```


### `libs\llm\ui\chat\tsconfig.spec.json`
```
{
  "extends": "./tsconfig.json",
  "compilerOptions": {
    "outDir": "../../../../dist/out-tsc",
    "types": [
      "vitest/globals",
      "vitest/importMeta",
      "vite/client",
      "node",
      "vitest"
    ]
  },
  "include": [
    "vite.config.ts",
    "vite.config.mts",
    "vitest.config.ts",
    "vitest.config.mts",
    "src/**/*.test.ts",
    "src/**/*.spec.ts",
    "src/**/*.test.tsx",
    "src/**/*.spec.tsx",
    "src/**/*.test.js",
    "src/**/*.spec.js",
    "src/**/*.test.jsx",
    "src/**/*.spec.jsx",
    "src/**/*.d.ts"
  ],
  "files": ["src/test-setup.ts"]
}

```


### `libs\llm\ui\chat\vite.config.mts`
```
/// <reference types='vitest' />
import { defineConfig } from 'vite';
import angular from '@analogjs/vite-plugin-angular';

export default defineConfig(() => ({
  root: __dirname,
  cacheDir: '../../../../node_modules/.vite/libs/llm/ui/chat',
  resolve: {
    tsconfigPaths: true,
  },
  plugins: [angular()],
  test: {
    name: 'llm-ui-chat',
    watch: false,
    globals: true,
    environment: 'jsdom',
    include: ['{src,tests}/**/*.{test,spec}.{js,mjs,cjs,ts,mts,cts,jsx,tsx}'],
    setupFiles: ['src/test-setup.ts'],
    reporters: ['default'],
    coverage: {
      reportsDirectory: '../../../../coverage/libs/llm/ui/chat',
      provider: 'v8' as const,
    },
  },
}));
```


### `libs\llm\ui\chat\src\index.ts`
```
export * from './lib/chat-feed/chat-feed.component';
export * from './lib/chat-input/chat-input.component';
export * from './lib/chat-strategy-prompt/chat-strategy-prompt.component';
export * from './lib/chat-review-prompt/chat-review-prompt.component';
export * from './lib/flow-tracker/flow-tracker.component';
export * from './lib/chat-flow-card/chat-flow-card.component';
export * from './lib/flow-inspector/flow-inspector.component';
export * from './lib/chat-header/chat-header.component';
```


### `libs\llm\ui\chat\src\test-setup.ts`
```
import '@angular/compiler';
import '@analogjs/vitest-angular/setup-snapshots';
import { setupTestBed } from '@analogjs/vitest-angular/setup-testbed';

setupTestBed({ zoneless: false });

```


### `libs\llm\ui\chat\src\lib\chat-feed\chat-feed.component.spec.ts`
```
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ChatFeedComponent } from './chat-feed.component';
import { ComponentRef } from '@angular/core';
import { describe, beforeEach, it, expect } from 'vitest';

describe('ChatFeedComponent', () => {
  let component: ChatFeedComponent;
  let fixture: ComponentFixture<ChatFeedComponent>;
  let componentRef: ComponentRef<ChatFeedComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ChatFeedComponent]
    }).compileComponents();

    fixture = TestBed.createComponent(ChatFeedComponent);
    component = fixture.componentInstance;
    componentRef = fixture.componentRef;
    
    // Set required signal input
    componentRef.setInput('feed', [
      { id: '1', source: 'user', content: 'Test prompt' },
      { id: '2', source: 'model', content: 'Test response' }
    ]);
    fixture.detectChanges();
  });

  it('should create the component', () => {
    expect(component).toBeTruthy();
  });

  it('should render feed items', () => {
    const compiled = fixture.nativeElement as HTMLElement;
    const strongTags = compiled.querySelectorAll('strong');
    
    expect(strongTags.length).toBe(2);
    expect(strongTags[0].textContent).toContain('user');
    expect(strongTags[1].textContent).toContain('model');
  });
});
```


### `libs\llm\ui\chat\src\lib\chat-feed\chat-feed.component.ts`
```
import { Component, input, output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ChatItem } from '@org/llm-state-chat';
import { ChatFlowCardComponent } from '../chat-flow-card/chat-flow-card.component';

@Component({
  selector: 'llm-chat-feed',
  standalone: true,
  imports: [CommonModule, ChatFlowCardComponent],
  template: `
    <div style="display: flex; flex-direction: column; gap: 8px;">
      @for (item of feed(); track item.id) {
        @if (item.source === 'flow_card' && item.flowId) {
          <llm-chat-flow-card 
            [flowId]="item.flowId" 
            [status]="item.status || 'completed'"
            (inspect)="inspectFlow.emit($event)">
          </llm-chat-flow-card>
        } @else {
          <div style="padding: 8px; border-radius: 4px; background: #f0f0f0;">
            <strong style="text-transform: uppercase; font-size: 0.8em; color: #555;">{{ item.source }}</strong>
            <pre style="margin: 4px 0 0; white-space: pre-wrap; font-family: monospace;">{{ item.content }}</pre>
          </div>
        }
      }
    </div>
  `
})
export class ChatFeedComponent {
  feed = input.required<ChatItem[]>();
  inspectFlow = output<string>();
}
```


### `libs\llm\ui\chat\src\lib\chat-input\chat-input.component.spec.ts`
```
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ChatInputComponent } from './chat-input.component';
import { vi, describe, beforeEach, it, expect } from 'vitest';

describe('ChatInputComponent', () => {
  let component: ChatInputComponent;
  let fixture: ComponentFixture<ChatInputComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ChatInputComponent]
    }).compileComponents();

    fixture = TestBed.createComponent(ChatInputComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should emit sendPrompt and clear input on submit', () => {
    const emitSpy = vi.spyOn(component.sendPrompt, 'emit');
    
    component.promptText.set(' Hello World ');
    component.submit();

    expect(emitSpy).toHaveBeenCalledWith('Hello World');
    expect(component.promptText()).toBe('');
  });

  it('should not emit if input is empty', () => {
    const emitSpy = vi.spyOn(component.sendPrompt, 'emit');
    
    component.promptText.set('   ');
    component.submit();

    expect(emitSpy).not.toHaveBeenCalled();
  });
});
```


### `libs\llm\ui\chat\src\lib\chat-input\chat-input.component.ts`
```
import { Component, output } from '@angular/core';

@Component({
  selector: 'llm-chat-input',
  standalone: true,
  template: `
    <div style="display: flex; gap: 8px; margin-top: 16px;">
      <input 
        #promptInput
        type="text" 
        (keyup.enter)="submit(promptInput.value); promptInput.value=''"
        placeholder="Type your message..." 
        style="flex: 1; padding: 8px; border: 1px solid #ccc; border-radius: 4px;"
      />
      <button 
        (click)="submit(promptInput.value); promptInput.value=''" 
        style="padding: 8px 16px; cursor: pointer;">
        Send
      </button>
    </div>
  `
})
export class ChatInputComponent {
  sendPrompt = output<string>();

  submit(rawText: string) {
    const text = rawText.trim();
    if (text) {
      this.sendPrompt.emit(text);
    }
  }
}
```


### `libs\llm\ui\chat\src\lib\chat-review-prompt\chat-review-prompt.component.spec.ts`
```
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ChatReviewPromptComponent } from './chat-review-prompt.component';
import { ComponentRef } from '@angular/core';
import { vi, describe, beforeEach, it, expect } from 'vitest';

describe('ChatReviewPromptComponent', () => {
  let component: ChatReviewPromptComponent;
  let fixture: ComponentFixture<ChatReviewPromptComponent>;
  let componentRef: ComponentRef<ChatReviewPromptComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ChatReviewPromptComponent]
    }).compileComponents();

    fixture = TestBed.createComponent(ChatReviewPromptComponent);
    component = fixture.componentInstance;
    componentRef = fixture.componentRef;
    
    componentRef.setInput('branch', 'candidate/test-123');
    fixture.detectChanges();
  });

  it('should emit true when accepted', () => {
    const emitSpy = vi.spyOn(component.reviewDecided, 'emit');
    component.decide(true);
    expect(emitSpy).toHaveBeenCalledWith(true);
  });

  it('should emit false when rejected', () => {
    const emitSpy = vi.spyOn(component.reviewDecided, 'emit');
    component.decide(false);
    expect(emitSpy).toHaveBeenCalledWith(false);
  });
});
```


### `libs\llm\ui\chat\src\lib\chat-review-prompt\chat-review-prompt.component.ts`
```
import { Component, input, output } from '@angular/core';

@Component({
  selector: 'llm-chat-review-prompt',
  standalone: true,
  template: `
    <div style="background: #d1e7dd; padding: 16px; border: 1px solid #a3cfbb; border-radius: 4px; margin-top: 16px;">
      <strong>👀 Previewing {{ branch() }}...</strong>
      <p style="margin: 8px 0; font-size: 0.9em; font-family: monospace;">📂 Files have been checked out locally. Open IDE to inspect.</p>
      <div style="display: flex; gap: 8px; margin-top: 12px;">
        <button (click)="decide(true)" style="padding: 6px 12px; cursor: pointer; background: #198754; color: white; border: none; border-radius: 4px;">Accept (y)</button>
        <button (click)="decide(false)" style="padding: 6px 12px; cursor: pointer; background: #dc3545; color: white; border: none; border-radius: 4px;">Reject (n)</button>
      </div>
    </div>
  `
})
export class ChatReviewPromptComponent {
  branch = input.required<string>();
  reviewDecided = output<boolean>();

  decide(accepted: boolean) {
    this.reviewDecided.emit(accepted);
  }
}
```


### `libs\llm\ui\chat\src\lib\chat-strategy-prompt\chat-strategy-prompt.component.spec.ts`
```
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ChatStrategyPromptComponent } from './chat-strategy-prompt.component';
import { DomainDelegationStrategy } from '@org/llm-core-facade';
import { vi, describe, beforeEach, it, expect } from 'vitest';

describe('ChatStrategyPromptComponent', () => {
  let component: ChatStrategyPromptComponent;
  let fixture: ComponentFixture<ChatStrategyPromptComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ChatStrategyPromptComponent]
    }).compileComponents();

    fixture = TestBed.createComponent(ChatStrategyPromptComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should emit the selected strategy', () => {
    const emitSpy = vi.spyOn(component.strategySelected, 'emit');
    
    component.select(DomainDelegationStrategy.REFINE);
    
    expect(emitSpy).toHaveBeenCalledWith(DomainDelegationStrategy.REFINE);
  });
});
```


### `libs\llm\ui\chat\src\lib\chat-strategy-prompt\chat-strategy-prompt.component.ts`
```
import { Component, output } from '@angular/core';
import { DomainDelegationStrategy } from '@org/llm-core-facade';

@Component({
  selector: 'llm-chat-strategy-prompt',
  standalone: true,
  template: `
    <div style="background: #fff3cd; padding: 16px; border: 1px solid #ffe69c; border-radius: 4px; margin-top: 16px;">
      <strong>Select Next Step:</strong>
      <div style="display: flex; gap: 8px; margin-top: 12px; flex-wrap: wrap;">
        <button (click)="select(DomainDelegationStrategy.MANUAL)" style="padding: 6px 12px; cursor: pointer;">[1] Manual Review</button>
        <button (click)="select(DomainDelegationStrategy.REVIEW)" style="padding: 6px 12px; cursor: pointer;">[2] Assisted Review</button>
        <button (click)="select(DomainDelegationStrategy.REFINE)" style="padding: 6px 12px; cursor: pointer;">[3] Auto-Refine</button>
        <button (click)="select(DomainDelegationStrategy.SKIP)" style="padding: 6px 12px; cursor: pointer;">[0] Skip / Abort</button>
      </div>
    </div>
  `
})
export class ChatStrategyPromptComponent {
  DomainDelegationStrategy = DomainDelegationStrategy;
  strategySelected = output<DomainDelegationStrategy>();

  select(strategy: DomainDelegationStrategy) {
    this.strategySelected.emit(strategy);
  }
}
```


### `libs\llm\ui\chat\src\lib\flow-inspector\flow-inspector.component.html`
```
<!-- Backdrop -->
<div 
  (click)="close.emit()"
  style="position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.5); z-index: 1000;">
</div>

<!-- Slide-out Pane -->
<div 
  style="position: fixed; top: 0; right: 0; bottom: 0; width: 75vw; max-width: 1200px; background: #fff; box-shadow: -5px 0 25px rgba(0,0,0,0.1); z-index: 1001; display: flex; flex-direction: column; animation: slideIn 0.3s ease-out;">
  
  <!-- Header -->
  <div style="padding: 16px 24px; border-bottom: 1px solid #dee2e6; background: #f8f9fa; display: flex; justify-content: space-between; align-items: flex-start;">
    <div>
      <div style="font-size: 12px; color: #868e96; font-family: monospace; margin-bottom: 4px;">{{ parsedReceipt()?.flowId }}</div>
      <h2 style="margin: 0 0 8px 0; font-size: 18px; color: #212529;">{{ parsedReceipt()?.task }}</h2>
      <div style="font-size: 14px; color: #495057;">{{ parsedReceipt()?.summary }}</div>
    </div>
    <button 
      (click)="close.emit()" 
      style="background: none; border: none; font-size: 24px; color: #adb5bd; cursor: pointer; padding: 0; line-height: 1;">
      &times;
    </button>
  </div>

  <!-- Agent Tabs -->
  <div style="display: flex; gap: 8px; padding: 12px 24px; border-bottom: 1px solid #dee2e6; background: #fff;">
    @for (agent of parsedReceipt()?.agents; track agent.agentId) {
      <button 
        (click)="selectedAgentId.set(agent.agentId)"
        [style.background]="selectedAgentId() === agent.agentId ? '#e7f5ff' : '#f8f9fa'"
        [style.border]="selectedAgentId() === agent.agentId ? '1px solid #339af0' : '1px solid #dee2e6'"
        [style.color]="selectedAgentId() === agent.agentId ? '#1971c2' : '#495057'"
        style="padding: 8px 16px; border-radius: 4px; cursor: pointer; font-size: 13px; font-weight: bold; display: flex; align-items: center; gap: 6px;">
        {{ agent.passed ? '✅' : '❌' }} {{ agent.agentId }}
      </button>
    }
  </div>

  <!-- Split View Area -->
  @if (activeAgent()) {
    <div style="display: flex; flex: 1; overflow: hidden; background: #f1f3f5;">
      
      <!-- Left Pane: Input & Output -->
      <div style="flex: 1; display: flex; flex-direction: column; border-right: 1px solid #dee2e6; background: #fff; overflow-y: auto;">
        <div style="padding: 16px;">
          <h4 style="margin: 0 0 12px 0; font-size: 12px; text-transform: uppercase; color: #868e96; letter-spacing: 0.5px;">Instruction</h4>
          <div style="background: #f8f9fa; padding: 12px; border-radius: 6px; font-size: 13px; color: #212529; white-space: pre-wrap;">{{ activeAgent()?.instruction }}</div>
        </div>
        <div style="padding: 16px; border-top: 1px solid #dee2e6;">
          <h4 style="margin: 0 0 12px 0; font-size: 12px; text-transform: uppercase; color: #868e96; letter-spacing: 0.5px;">Raw Payload</h4>
          <pre style="background: #212529; color: #f8f9fa; padding: 16px; border-radius: 6px; font-size: 12px; overflow-x: auto; margin: 0;">{{ activeAgent()?.rawPayload }}</pre>
        </div>
      </div>

      <!-- Right Pane: Traces & Deltas -->
      <div style="flex: 1; display: flex; flex-direction: column; background: #fff; overflow-y: auto;">
        <div style="padding: 16px;">
          <h4 style="margin: 0 0 12px 0; font-size: 12px; text-transform: uppercase; color: #868e96; letter-spacing: 0.5px;">Verification Trace</h4>
          @if (activeAgent()?.verificationTrace) {
            <pre style="background: #fff5f5; color: #e03131; border: 1px solid #ffa8a8; padding: 12px; border-radius: 6px; font-size: 12px; overflow-x: auto; margin: 0; white-space: pre-wrap;">{{ activeAgent()?.verificationTrace }}</pre>
          } @else {
            <div style="color: #adb5bd; font-size: 13px; font-style: italic;">No trace output.</div>
          }
        </div>
        <div style="padding: 16px; border-top: 1px solid #dee2e6; flex: 1;">
          <h4 style="margin: 0 0 12px 0; font-size: 12px; text-transform: uppercase; color: #868e96; letter-spacing: 0.5px;">State Delta (Git Patch)</h4>
          @if (activeAgent()?.stateDelta) {
            <pre style="background: #f4fce3; color: #2b8a3e; border: 1px solid #b2f2bb; padding: 12px; border-radius: 6px; font-size: 12px; overflow-x: auto; margin: 0;">{{ activeAgent()?.stateDelta }}</pre>
          } @else {
            <div style="color: #adb5bd; font-size: 13px; font-style: italic;">No file changes proposed.</div>
          }
        </div>
      </div>

    </div>
  }
</div>

<style>
  @keyframes slideIn {
    from { transform: translateX(100%); }
    to { transform: translateX(0); }
  }
</style>
```


### `libs\llm\ui\chat\src\lib\flow-inspector\flow-inspector.component.spec.ts`
```
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { FlowInspectorComponent } from './flow-inspector.component';
import { ComponentRef } from '@angular/core';
import { vi, describe, beforeEach, it, expect } from 'vitest';

const mockXml = `<?xml version="1.0" encoding="UTF-8"?>
<FlowReceipt FlowID="flow-999" TaskID="task-1" Timestamp="2026-09-02T12:00:00Z">
  <Task>Test Task</Task>
  <Summary>Task Summary</Summary>
  <Agents>
    <Agent AgentID="agent-1" Passed="true">
      <Instruction>Do A</Instruction>
      <RawPayload>Output A</RawPayload>
      <VerificationTrace>Trace A</VerificationTrace>
      <StateDelta>Delta A</StateDelta>
    </Agent>
    <Agent AgentID="agent-2" Passed="false">
      <Instruction>Do B</Instruction>
      <RawPayload>Output B</RawPayload>
      <VerificationTrace>Trace B</VerificationTrace>
      <StateDelta>Delta B</StateDelta>
    </Agent>
  </Agents>
</FlowReceipt>`;

describe('FlowInspectorComponent', () => {
  let component: FlowInspectorComponent;
  let fixture: ComponentFixture<FlowInspectorComponent>;
  let componentRef: ComponentRef<FlowInspectorComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [FlowInspectorComponent]
    }).compileComponents();

    fixture = TestBed.createComponent(FlowInspectorComponent);
    component = fixture.componentInstance;
    componentRef = fixture.componentRef;
    
    componentRef.setInput('receiptXml', mockXml);
    fixture.detectChanges();
  });

  it('should parse XML and set the first agent as active', () => {
    const receipt = component.parsedReceipt();
    
    expect(receipt).toBeTruthy();
    expect(receipt?.flowId).toBe('flow-999');
    expect(receipt?.task).toBe('Test Task');
    expect(receipt?.agents.length).toBe(2);
    
    expect(component.selectedAgentId()).toBe('agent-1');
    expect(component.activeAgent()?.instruction).toBe('Do A');
  });

  it('should change active agent when selected', () => {
    component.selectedAgentId.set('agent-2');
    fixture.detectChanges();
    
    expect(component.activeAgent()?.instruction).toBe('Do B');
    expect(component.activeAgent()?.passed).toBe(false);
  });

  it('should emit close event when close button is clicked', () => {
    const emitSpy = vi.spyOn(component.close, 'emit');
    
    // Select the close button specifically
    const closeButton = fixture.nativeElement.querySelector('button');
    closeButton.click();
    
    expect(emitSpy).toHaveBeenCalled();
  });
});
```


### `libs\llm\ui\chat\src\lib\flow-inspector\flow-inspector.component.ts`
```
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
```


### `libs\llm\ui\chat\src\lib\flow-tracker\flow-tracker.component.html`
```
<div style="display: flex; flex-direction: column; gap: 12px; padding: 12px; background: #f8f9fa; border-top: 1px solid #dee2e6; height: 100%; overflow-y: auto;">
  <h3 style="margin: 0; font-size: 14px; color: #495057; text-transform: uppercase; letter-spacing: 0.5px;">
    🏭 Execution Telemetry
  </h3>
  
  @if (flowsArray().length === 0) {
    <div style="color: #adb5bd; font-size: 13px; font-style: italic;">Awaiting orchestration...</div>
  }

  @for (flow of flowsArray(); track flow.flowId) {
    <div style="border: 1px solid #ced4da; border-radius: 6px; background: #ffffff; padding: 12px; margin-bottom: 8px;">
      
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; border-bottom: 1px solid #f1f3f5; padding-bottom: 8px;">
        <strong style="font-size: 13px; color: #343a40;">
          Flow: {{ flow.flowId.split('-').pop() }} 
          <span style="font-weight: normal; color: #868e96;">({{ flow.agentCount }} agents)</span>
        </strong>
        <span [style.color]="flow.status === 'running' ? '#f59f00' : '#2b8a3e'" style="font-size: 12px; font-weight: bold; text-transform: uppercase;">
          {{ flow.status === 'running' ? '⏳ Running' : '✅ Completed' }}
        </span>
      </div>

      <div style="display: flex; flex-direction: column; gap: 12px;">
        @for (agent of flow.agentsArray; track agent.agentId) {
          
          <!-- Terminal Block -->
          <div style="border: 1px solid #343a40; border-radius: 4px; background: #212529; color: #f8f9fa; font-family: monospace; font-size: 12px; overflow: hidden;">
            
            <div style="background: #111; padding: 6px 12px; display: flex; justify-content: space-between; border-bottom: 1px solid #343a40;">
              <span style="font-weight: bold; color: #4dabf7;">Agent {{ agent.agentIndex }}</span>
              <span>{{ agent.passed ? '✅ Passed' : (agent.trace ? '❌ Errored' : '⏳ Running') }}</span>
            </div>
            
            <div style="padding: 8px 12px; color: #adb5bd; border-bottom: 1px dashed #495057; white-space: pre-wrap; font-size: 11px;">{{ agent.instruction }}</div>
            
            <div style="padding: 8px 12px; display: flex; flex-direction: column; gap: 4px; max-height: 300px; overflow-y: auto;">
              @if (agent.timeline.length === 0) {
                <span style="color: #868e96; font-style: italic;">Waiting for events...</span>
              }
              @for (entry of agent.timeline; track entry.id) {
                <div [style.color]="entry.isError ? '#ff8787' : '#b2f2bb'" style="word-break: break-all; margin-bottom: 2px;">
                  <span style="color: #868e96; margin-right: 6px;">[{{ entry.timestamp | date:'HH:mm:ss' }}]</span>
                  <span [style.font-weight]="entry.isError ? 'bold' : 'normal'">{{ entry.message }}</span>
                </div>
              }
            </div>

          </div>

        }
      </div>
    </div>
  }
</div>
```


### `libs\llm\ui\chat\src\lib\flow-tracker\flow-tracker.component.spec.ts`
```
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { FlowTrackerComponent } from './flow-tracker.component';
import { ComponentRef } from '@angular/core';
import { describe, beforeEach, it, expect } from 'vitest';
import { FlowState } from '@org/llm-state-chat';

describe('FlowTrackerComponent', () => {
  let component: FlowTrackerComponent;
  let fixture: ComponentFixture<FlowTrackerComponent>;
  let componentRef: ComponentRef<FlowTrackerComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [FlowTrackerComponent]
    }).compileComponents();

    fixture = TestBed.createComponent(FlowTrackerComponent);
    component = fixture.componentInstance;
    componentRef = fixture.componentRef;
    
    const mockMap = new Map<string, FlowState>();
    mockMap.set('flow-1', {
      flowId: 'flow-1',
      taskId: 'task-1',
      agentCount: 1,
      completedCount: 0,
      status: 'running',
      agents: new Map([
        ['agent-1', {
          agentId: 'agent-1',
          agentIndex: 1,
          instruction: 'Write a loop',
          status: 'running_tests',
          attempt: 1,
          trace: '',
          passed: false,
          timeline: [
            { id: '1', timestamp: Date.now(), message: '[Attempt 1] Status: running_tests', isError: false }
          ]
        }]
      ])
    });

    componentRef.setInput('flows', mockMap);
    fixture.detectChanges();
  });

  it('should compute flowsArray sorted by running status', () => {
    const flows = component.flowsArray();
    expect(flows.length).toBe(1);
    expect(flows[0].flowId).toBe('flow-1');
    expect(flows[0].agentsArray.length).toBe(1);
    expect(flows[0].agentsArray[0].agentId).toBe('agent-1');
  });

  it('should render the active flows as a terminal feed', () => {
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.textContent).toContain('Flow: 1');
    expect(compiled.textContent).toContain('Agent 1');
    expect(compiled.textContent).toContain('Write a loop');
    expect(compiled.textContent).toContain('[Attempt 1] Status: running_tests');
  });
});
```


### `libs\llm\ui\chat\src\lib\flow-tracker\flow-tracker.component.ts`
```
import { Component, input, computed } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FlowState } from '@org/llm-state-chat';

@Component({
  selector: 'llm-flow-tracker',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './flow-tracker.component.html'
})
export class FlowTrackerComponent {
  flows = input.required<Map<string, FlowState>>();

  flowsArray = computed(() => {
    return Array.from(this.flows().values())
      .map(flow => ({
        ...flow,
        agentsArray: Array.from(flow.agents.values()).sort((a, b) => a.agentIndex - b.agentIndex)
      }))
      .sort((a, b) => a.status === 'running' ? -1 : 1); 
  });
}
```


### `libs\llm\ui\chat\src\lib\chat-flow-card\chat-flow-card.component.spec.ts`
```
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ChatFlowCardComponent } from './chat-flow-card.component';
import { ComponentRef } from '@angular/core';
import { vi, describe, beforeEach, it, expect } from 'vitest';

describe('ChatFlowCardComponent', () => {
  let component: ChatFlowCardComponent;
  let fixture: ComponentFixture<ChatFlowCardComponent>;
  let componentRef: ComponentRef<ChatFlowCardComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ChatFlowCardComponent]
    }).compileComponents();

    fixture = TestBed.createComponent(ChatFlowCardComponent);
    component = fixture.componentInstance;
    componentRef = fixture.componentRef;
    
    componentRef.setInput('flowId', 'flow-123');
    fixture.detectChanges();
  });

  it('should display the flow ID', () => {
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.textContent).toContain('flow-123');
  });

  it('should emit inspect event with flowId on click', () => {
    const emitSpy = vi.spyOn(component.inspect, 'emit');
    const button = fixture.nativeElement.querySelector('button');
    
    button.click();
    
    expect(emitSpy).toHaveBeenCalledWith('flow-123');
  });
});
```


### `libs\llm\ui\chat\src\lib\chat-flow-card\chat-flow-card.component.ts`
```
import { Component, input, output } from '@angular/core';

@Component({
  selector: 'llm-chat-flow-card',
  standalone: true,
  template: `
    <div style="background: #e9ecef; border: 1px solid #ced4da; border-radius: 8px; padding: 12px; margin: 8px 0; display: flex; justify-content: space-between; align-items: center;">
      <div>
        <strong style="color: #495057; display: block; font-size: 14px;">
          {{ status() === 'running' ? '⏳' : '✅' }} Orchestration Flow
        </strong>
        <span style="color: #868e96; font-size: 12px; font-family: monospace;">{{ flowId() }}</span>
      </div>
      <button 
        (click)="inspect.emit(flowId())"
        style="background: #fff; border: 1px solid #adb5bd; padding: 6px 12px; border-radius: 4px; cursor: pointer; font-size: 12px; font-weight: bold; color: #495057; transition: background 0.2s;"
        onmouseover="this.style.background='#f8f9fa'" 
        onmouseout="this.style.background='#fff'">
        🔍 Inspect Details
      </button>
    </div>
  `
})
export class ChatFlowCardComponent {
  flowId = input.required<string>();
  status = input<'running' | 'completed'>('completed');
  inspect = output<string>();
}
```


### `libs\llm\ui\chat\src\lib\chat-header\chat-header.component.spec.ts`
```
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ComponentRef } from '@angular/core';
import { ChatHeaderComponent } from './chat-header.component';
import { vi, describe, beforeEach, it, expect } from 'vitest';

describe('ChatHeaderComponent', () => {
  let component: ChatHeaderComponent;
  let fixture: ComponentFixture<ChatHeaderComponent>;
  let componentReference: ComponentRef<ChatHeaderComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ChatHeaderComponent]
    }).compileComponents();

    fixture = TestBed.createComponent(ChatHeaderComponent);
    component = fixture.componentInstance;
    componentReference = fixture.componentRef;
    
    componentReference.setInput('spaces', [
      { id: 'golang', name: 'Go Developer', isConfigured: true }
    ]);
    componentReference.setInput('activeSpaceId', 'golang');
    
    componentReference.setInput('chats', [
      { id: 'chat-1', name: 'First Chat', createdAt: '2026-09-07T00:00:00Z' },
      { id: 'chat-2', name: 'Second Chat', createdAt: '2026-09-07T01:00:00Z' }
    ]);
    componentReference.setInput('activeChatId', 'chat-1');

    fixture.detectChanges();
  });

  it('should emit spaceSelected when a new space is chosen', () => {
    const emitSpy = vi.spyOn(component.spaceSelected, 'emit');
    const selectElement = fixture.nativeElement.querySelector('#spaceSelect') as HTMLSelectElement;
    
    selectElement.value = 'golang';
    selectElement.dispatchEvent(new Event('change'));
    
    expect(emitSpy).toHaveBeenCalledWith('golang');
  });

  it('should emit chatSelected when a new chat is chosen', () => {
    const emitSpy = vi.spyOn(component.chatSelected, 'emit');
    const selectElement = fixture.nativeElement.querySelector('#chatSelect') as HTMLSelectElement;
    
    selectElement.value = 'chat-2';
    selectElement.dispatchEvent(new Event('change'));
    
    expect(emitSpy).toHaveBeenCalledWith('chat-2');
  });

  it('should emit newChatRequested when the new button is clicked', () => {
    const emitSpy = vi.spyOn(component.newChatRequested, 'emit');
    const newButton = fixture.nativeElement.querySelector('button') as HTMLButtonElement;
    
    newButton.click();
    
    expect(emitSpy).toHaveBeenCalled();
  });
});
```


### `libs\llm\ui\chat\src\lib\chat-header\chat-header.component.ts`
```
import { Component, input, output } from '@angular/core';
import { CommonModule } from '@angular/common';

export interface HeaderSpace {
  id: string;
  name: string;
  isConfigured: boolean;
}

export interface HeaderChat {
  id: string;
  name: string;
  createdAt: string;
}

@Component({
  selector: 'llm-chat-header',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div style="display: flex; gap: 24px; align-items: center; width: 100%;">
      <div style="display: flex; flex-direction: column; gap: 4px; flex: 1;">
        <label for="spaceSelect" style="font-size: 11px; color: #868e96; font-weight: bold; text-transform: uppercase; letter-spacing: 0.5px;">ThinkSpace</label>
        <select 
          id="spaceSelect" 
          [value]="activeSpaceId() || ''" 
          (change)="handleSpaceChange($event)" 
          style="padding: 8px; border: 1px solid #ced4da; border-radius: 4px; background: white; font-size: 14px; width: 100%;">
          <option value="" disabled>Select Space...</option>
          @for (space of spaces(); track space.id) {
            <option [value]="space.id">{{ space.name }} {{ space.isConfigured ? '' : '(Unconfigured)' }}</option>
          }
        </select>
      </div>

      <div style="display: flex; flex-direction: column; gap: 4px; flex: 2;">
        <label for="chatSelect" style="font-size: 11px; color: #868e96; font-weight: bold; text-transform: uppercase; letter-spacing: 0.5px;">Conversation Thread</label>
        <div style="display: flex; gap: 8px;">
          <select 
            id="chatSelect" 
            [value]="activeChatId() || ''" 
            (change)="handleChatChange($event)" 
            style="padding: 8px; border: 1px solid #ced4da; border-radius: 4px; background: white; font-size: 14px; width: 100%;">
            <option value="" disabled>Select Chat...</option>
            @for (chat of chats(); track chat.id) {
              <option [value]="chat.id">{{ chat.name }}</option>
            }
          </select>
          <button 
            (click)="newChatRequested.emit()" 
            style="padding: 8px 16px; background: #f8f9fa; border: 1px solid #ced4da; border-radius: 4px; cursor: pointer; font-weight: bold; color: #495057; white-space: nowrap;">
            + New
          </button>
        </div>
      </div>
    </div>
  `
})
export class ChatHeaderComponent {
  public spaces = input.required<HeaderSpace[]>();
  public activeSpaceId = input.required<string | null>();
  
  public chats = input.required<HeaderChat[]>();
  public activeChatId = input.required<string | null>();

  public spaceSelected = output<string>();
  public chatSelected = output<string>();
  public newChatRequested = output<void>();

  public handleSpaceChange(event: Event): void {
    const targetElement = event.target as HTMLSelectElement;
    if (targetElement.value) {
      this.spaceSelected.emit(targetElement.value);
    }
  }

  public handleChatChange(event: Event): void {
    const targetElement = event.target as HTMLSelectElement;
    if (targetElement.value) {
      this.chatSelected.emit(targetElement.value);
    }
  }
}
```
