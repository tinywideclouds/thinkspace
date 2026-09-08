

### `libs\llm\state\chat\eslint.config.mjs`
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


### `libs\llm\state\chat\project.json`
```
{
  "name": "llm-state-chat",
  "$schema": "../../../../node_modules/nx/schemas/project-schema.json",
  "sourceRoot": "libs/llm/state/chat/src",
  "prefix": "lib",
  "projectType": "library",
  "tags": ["scope:llm"],
  "targets": {
    "lint": {
      "executor": "@nx/eslint:lint"
    }
  }
}

```


### `libs\llm\state\chat\README.md`
```
# llm-state-chat

This library acts as the central "brain" of the ThinkSpace web UI. It maps domain events to reactive Angular Signals, maintaining the state machine of the multi-agent CLI loop.

## Responsibilities
* Connect to the `TransportService` and route domain events through the `LlmFacade`.
* Manage partitioned UI state using modern Angular Signals (`coreChat`, `systemLogs`, `activeAgents`).
* Handle blocking workflow states (`pendingStrategy`, `pendingReviewBranch`) replicating the synchronous terminal experience.

## Rules
* **MUST** use `inject()` for all dependency resolution.
* **MUST NOT** include any HTML templates or UI rendering logic.
* Operates exclusively on Domain types imported from `@org/llm-core-facade`.
```


### `libs\llm\state\chat\tsconfig.json`
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


### `libs\llm\state\chat\tsconfig.lib.json`
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


### `libs\llm\state\chat\tsconfig.spec.json`
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


### `libs\llm\state\chat\vite.config.mts`
```
/// <reference types='vitest' />
import { defineConfig } from 'vite';
import angular from '@analogjs/vite-plugin-angular';

export default defineConfig(() => ({
  root: __dirname,
  cacheDir: '../../../../node_modules/.vite/libs/llm/state/chat',
  resolve: {
    tsconfigPaths: true,
  },
  plugins: [angular()],
  test: {
    name: 'llm-state-chat',
    watch: false,
    globals: true,
    environment: 'jsdom',
    include: ['{src,tests}/**/*.{test,spec}.{js,mjs,cjs,ts,mts,cts,jsx,tsx}'],
    setupFiles: ['src/test-setup.ts'],
    reporters: ['default'],
    coverage: {
      reportsDirectory: '../../../../coverage/libs/llm/state/chat',
      provider: 'v8' as const,
    },
  },
}));
```


### `libs\llm\state\chat\src\index.ts`
```
export * from './lib/chat-state.service';
export * from './lib/workspace-state.service';
```


### `libs\llm\state\chat\src\test-setup.ts`
```
import '@angular/compiler';
import '@analogjs/vitest-angular/setup-snapshots';
import { setupTestBed } from '@analogjs/vitest-angular/setup-testbed';

setupTestBed({ zoneless: false });

```


### `libs\llm\state\chat\src\lib\chat-state.service.spec.ts`
```
import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { ChatStateService } from './chat-state.service';
import { WorkspaceStateService } from './workspace-state.service';
import { TransportService } from '@org/llm-core-transport';
import { DomainDelegationStrategy } from '@org/llm-core-facade';
import { WSEvent, WSEventSchema } from '@org/llm-core-protos';
import { create } from '@bufbuild/protobuf';
import { Subject } from 'rxjs';
import { signal } from '@angular/core';
import { describe, beforeEach, it, expect, vi, Mock } from 'vitest';

describe('ChatStateService', () => {
  let service: ChatStateService;
  let httpTestingController: HttpTestingController;
  let mockTransportService: { connect: Mock, send: Mock, disconnect: Mock };
  let mockWorkspaceStateService: any;
  let webSocketSubject: Subject<WSEvent>;

  beforeEach(() => {
    webSocketSubject = new Subject<WSEvent>();
    
    mockTransportService = {
      connect: vi.fn(() => webSocketSubject.asObservable()),
      send: vi.fn(),
      disconnect: vi.fn(),
    };

    mockWorkspaceStateService = {
      activeSpaceId: signal('golang'),
      activeChatId: signal('test-chat')
    };

    TestBed.configureTestingModule({
      providers: [
        ChatStateService,
        provideHttpClient(),
        provideHttpClientTesting(),
        { provide: TransportService, useValue: mockTransportService },
        { provide: WorkspaceStateService, useValue: mockWorkspaceStateService }
      ]
    });
    
    service = TestBed.inject(ChatStateService);
    httpTestingController = TestBed.inject(HttpTestingController);
  });

  it('should update connection status and initialize on connect', () => {
    service.connect('ws://test');
    expect(mockTransportService.connect).toHaveBeenCalledWith('ws://test');
    expect(service.connected()).toBe(true);
  });

  it('should stream model chunks into a single coreChat item', () => {
    service.connect('ws://test');
    
    const firstChunk = create(WSEventSchema, { payload: { case: 'chatStream', value: { text: 'Hello ' } } });
    const secondChunk = create(WSEventSchema, { payload: { case: 'chatStream', value: { text: 'World' } } });
    
    webSocketSubject.next(firstChunk);
    webSocketSubject.next(secondChunk);

    const chatHistory = service.coreChat();
    expect(chatHistory.length).toBe(1);
    expect(chatHistory[0].source).toBe('model');
    expect(chatHistory[0].content).toBe('Hello World');
  });

  it('should trigger pending strategy state on request_strategy', () => {
    service.connect('ws://test');
    
    const requestStrategyEvent = create(WSEventSchema, { payload: { case: 'requestStrategy', value: { active: true } } });
    webSocketSubject.next(requestStrategyEvent);

    expect(service.pendingStrategy()).toBe(true);
  });

  it('should trigger pending review state on request_review', () => {
    service.connect('ws://test');
    
    const requestReviewEvent = create(WSEventSchema, { payload: { case: 'requestReview', value: { branch: 'candidate/123' } } });
    webSocketSubject.next(requestReviewEvent);

    expect(service.pendingReviewBranch()).toBe('candidate/123');
  });

  it('should send prompt with correct space and chat ids', () => {
    service.submitPrompt('Refactor this app');
    
    expect(mockTransportService.send).toHaveBeenCalled();
    const chatHistory = service.coreChat();
    expect(chatHistory.length).toBe(2);
    expect(chatHistory[0].source).toBe('user');
    expect(chatHistory[0].content).toBe('Refactor this app');
    
    const sentProtocolBuffer = mockTransportService.send.mock.calls[0][0];
    expect(sentProtocolBuffer.payload.value.text).toBe('Refactor this app');
    expect(sentProtocolBuffer.payload.value.spaceId).toBe('golang');
    expect(sentProtocolBuffer.payload.value.chatId).toBe('test-chat');
  });

  it('should clear strategy state and send on selectStrategy', () => {
    service.pendingStrategy.set(true);
    service.selectStrategy(DomainDelegationStrategy.REFINE);
    
    expect(service.pendingStrategy()).toBe(false);
    expect(mockTransportService.send).toHaveBeenCalled();
  });

  it('should clear review state and send on submitReview', () => {
    service.pendingReviewBranch.set('candidate/123');
    service.submitReview(true);
    
    expect(service.pendingReviewBranch()).toBeNull();
    expect(mockTransportService.send).toHaveBeenCalled();
  });

  it('should clear the session completely on clearSession', () => {
    service.coreChat.set([{ id: '1', source: 'user', content: 'test' }]);
    service.pendingStrategy.set(true);
    service.inspectedReceipt.set('<xml/>');

    service.clearSession();

    expect(service.coreChat().length).toBe(0);
    expect(service.pendingStrategy()).toBe(false);
    expect(service.inspectedReceipt()).toBeNull();
  });

  it('should fetch flow receipt XML over HTTP with query parameters', async () => {
    const fetchPromise = service.inspectFlow('flow-999');
    
    const request = httpTestingController.expectOne('/api/receipts/test-chat/flow-999?space=golang');
    expect(request.request.method).toBe('GET');
    
    request.flush('<FlowReceipt FlowID="flow-999"></FlowReceipt>');
    
    await fetchPromise;
    expect(service.inspectedReceipt()).toBe('<FlowReceipt FlowID="flow-999"></FlowReceipt>');
  });

  it('should track agent stream by safely auto-initializing the activeAgents map', () => {
    service.connect('ws://test');
    
    // We removed agentStart, so we test that agentStream handles missing agent records cleanly
    webSocketSubject.next(create(WSEventSchema, { 
      payload: { case: 'agentStream', value: { agentId: 1, text: 'func main() {}' } } 
    }));

    const finalAgent = service.activeAgents().get(1);
    expect(finalAgent?.stream).toBe('func main() {}');
    expect(finalAgent?.status).toBe('running');
  });
});
```


### `libs\llm\state\chat\src\lib\chat-state.service.ts`
```
import { Injectable, signal, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { TransportService } from '@org/llm-core-transport';
import { LlmFacade, DomainEvent, DomainDelegationStrategy, DomainFlowEvent } from '@org/llm-core-facade';
import { firstValueFrom } from 'rxjs';
import { WorkspaceStateService } from './workspace-state.service';

export interface ChatItem {
  id: string;
  source: 'user' | 'model' | 'system' | 'flow_card';
  content: string;
  flowId?: string; 
}

export interface LogItem {
  id: string;
  timestamp: number;
  level: string;
  message: string;
}

export interface AgentState {
  id: number;
  instructions: string;
  stream: string;
  status: 'running' | 'completed';
  branch?: string;
  verified?: boolean;
}

export interface FlowAgentState {
  agentId: string;
  agentIndex: number;
  instruction: string;
  status: string;
  attempt: number;
  trace: string;
  passed: boolean;
}

export interface FlowState {
  flowId: string;
  taskId: string;
  agentCount: number;
  completedCount: number;
  status: 'running' | 'completed';
  agents: Map<string, FlowAgentState>;
}

@Injectable({ providedIn: 'root' })
export class ChatStateService {
  private transportService = inject(TransportService);
  private httpClient = inject(HttpClient);
  private workspaceStateService = inject(WorkspaceStateService); 

  public coreChat = signal<ChatItem[]>([]);
  public systemLogs = signal<LogItem[]>([]);
  public activeAgents = signal<Map<number, AgentState>>(new Map());
  public flowStates = signal<Map<string, FlowState>>(new Map());
  
  public pendingStrategy = signal<boolean>(false);
  public pendingReviewBranch = signal<string | null>(null);
  public connected = signal<boolean>(false);
  public inspectedReceipt = signal<string | null>(null);

  public connect(url: string): void {
    this.transportService.connect(url).subscribe({
      next: (protocolBufferEvent) => {
        const domainEvent = LlmFacade.toDomain(protocolBufferEvent);
        if (domainEvent) {
          this.handleEvent(domainEvent);
        }
      },
      error: (error) => {
        this.appendSystemLog('ERROR', `Connection error: ${error}`);
        this.connected.set(false);
      },
      complete: () => {
        this.appendSystemLog('INFO', 'Connection closed.');
        this.connected.set(false);
      }
    });
    this.connected.set(true);
  }

  public disconnect(): void {
    this.transportService.disconnect();
    this.connected.set(false);
  }

  public clearSession(): void {
    this.coreChat.set([]);
    this.systemLogs.set([]);
    this.activeAgents.set(new Map());
    this.flowStates.set(new Map());
    this.pendingStrategy.set(false);
    this.pendingReviewBranch.set(null);
    this.inspectedReceipt.set(null);
  }

  public submitPrompt(text: string): void {
    const spaceId = this.workspaceStateService.activeSpaceId();
    const chatId = this.workspaceStateService.activeChatId();

    if (!spaceId || !chatId) {
      this.appendSystemLog('ERROR', 'Cannot submit prompt: No active space or chat selected.');
      return;
    }

    this.appendCoreChat('user', text);
    // the llm system now gives back true state here instead of assuming this.
    // this.appendCoreChat('system', '🤖 Manager is thinking...');
    
    const protocolBufferEvent = LlmFacade.createSubmitPrompt(text, spaceId, chatId);
    this.transportService.send(protocolBufferEvent);
  }

  public async inspectFlow(flowId: string): Promise<void> {
    const spaceId = this.workspaceStateService.activeSpaceId();
    const chatId = this.workspaceStateService.activeChatId();

    if (!spaceId || !chatId) return;

    try {
      const url = `/api/receipts/${encodeURIComponent(chatId)}/${encodeURIComponent(flowId)}?space=${encodeURIComponent(spaceId)}`;
      const xmlData = await firstValueFrom(this.httpClient.get(url, { responseType: 'text' }));
      this.inspectedReceipt.set(xmlData);
    } catch (error) {
      console.error(`Failed to fetch receipt for flow ${flowId}`, error);
      this.appendSystemLog('ERROR', `Failed to load receipt for flow ${flowId}`);
    }
  }

  private handleEvent(event: DomainEvent): void {
    switch (event.type) {
      case 'available_spaces':
        // Legacy support: Handled by WorkspaceStateService
        break;
      case 'chat_stream':
        this.appendToLastModelMessage(event.text);
        break;
      case 'log_message':
        this.appendSystemLog(event.level, event.message);
        break;
      case 'flow_event':
        this.handleFlowEvent(event);
        break;
      case 'agent_stream':
        this.activeAgents.update(currentMap => {
          const updatedMap = new Map(currentMap);
          let agent = updatedMap.get(event.agentId);
          if (!agent) {
            agent = { id: event.agentId, instructions: '', stream: '', status: 'running' };
          }
          agent.stream += event.text;
          updatedMap.set(event.agentId, agent);
          return updatedMap;
        });
        break;
      case 'request_strategy':
        this.pendingStrategy.set(event.active);
        break;
      case 'request_review':
        this.pendingReviewBranch.set(event.branch);
        break;
    }
  }

  private handleFlowEvent(event: DomainFlowEvent): void {
    this.flowStates.update(currentFlows => {
      const updatedMap = new Map(currentFlows);
      let flow = updatedMap.get(event.flowId);

      if (event.eventType === 'flow_start') {
        flow = {
          flowId: event.flowId,
          taskId: event.taskId,
          agentCount: event.agentCount,
          completedCount: 0,
          status: 'running',
          agents: new Map()
        };
        updatedMap.set(event.flowId, flow);
      }

      if (event.eventType === 'flow_complete' && flow) {
        flow.completedCount++;
        if (flow.completedCount === flow.agentCount && flow.status !== 'completed') {
          flow.status = 'completed';
          this.coreChat.update(chatHistory => [...chatHistory, { 
            id: crypto.randomUUID(), 
            source: 'flow_card', 
            content: 'Orchestration Flow Completed', 
            flowId: event.flowId 
          }]);
        }
      }

      if (!flow) return updatedMap;

      if (event.agentId) {
        const agent = flow.agents.get(event.agentId) || {
          agentId: event.agentId,
          agentIndex: event.agentIndex,
          instruction: '',
          status: 'starting',
          attempt: 1,
          trace: '',
          passed: false
        };

        if (event.instruction) agent.instruction = event.instruction;
        if (event.status) agent.status = event.status;
        if (event.attempt) agent.attempt = event.attempt;
        if (event.trace) agent.trace = event.trace;
        if (event.passed !== undefined) agent.passed = event.passed;

        flow.agents.set(event.agentId, agent);
      }

      return updatedMap;
    });
  }

  private appendCoreChat(source: ChatItem['source'], content: string): void {
    this.coreChat.update(chatHistory => [...chatHistory, { id: crypto.randomUUID(), source, content }]);
  }

  private appendToLastModelMessage(chunkText: string): void {
    this.coreChat.update(chatHistory => {
      if (chatHistory.length === 0 || chatHistory[chatHistory.length - 1].source !== 'model') {
        return [...chatHistory, { id: crypto.randomUUID(), source: 'model', content: chunkText }];
      }
      const updatedChatHistory = [...chatHistory];
      updatedChatHistory[updatedChatHistory.length - 1].content += chunkText;
      return updatedChatHistory;
    });
  }

  private appendSystemLog(level: string, message: string): void {
    this.systemLogs.update(logs => [...logs, { id: crypto.randomUUID(), timestamp: Date.now(), level, message }]);
  }

  public selectStrategy(strategy: DomainDelegationStrategy): void {
    this.pendingStrategy.set(false);
    this.appendCoreChat('user', `Selected Strategy: ${DomainDelegationStrategy[strategy]}`);
    const protocolBufferEvent = LlmFacade.createSelectStrategy(strategy);
    this.transportService.send(protocolBufferEvent);
  }

  public submitReview(accepted: boolean): void {
    const branch = this.pendingReviewBranch();
    if (!branch) return;
    
    this.pendingReviewBranch.set(null);
    this.appendCoreChat('user', `Review for ${branch}: ${accepted ? 'ACCEPTED' : 'REJECTED'}`);
    const protocolBufferEvent = LlmFacade.createReviewDecision(branch, accepted);
    this.transportService.send(protocolBufferEvent);
  }
}
```
