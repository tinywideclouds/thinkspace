

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
import { nxViteTsPaths } from '@nx/vite/plugins/nx-tsconfig-paths.plugin';
import { nxCopyAssetsPlugin } from '@nx/vite/plugins/nx-copy-assets.plugin';

export default defineConfig(() => ({
  root: __dirname,
  cacheDir: '../../../../node_modules/.vite/libs/llm/state/chat',
  plugins: [angular(), nxViteTsPaths(), nxCopyAssetsPlugin(['*.md'])],
  // Uncomment this if you are using workers.
  // worker: {
  //   plugins: () => [ nxViteTsPaths() ],
  // },
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
import { ChatStateService } from './chat-state.service';
import { TransportService } from '@org/llm-core-transport';
import { DomainDelegationStrategy } from '@org/llm-core-facade';
import { WSEvent, WSEventSchema } from '@org/llm-core-protos';
import { create } from '@bufbuild/protobuf';
import { Subject } from 'rxjs';
import { describe, beforeEach, it, expect, vi, Mock } from 'vitest';

describe('ChatStateService', () => {
  let service: ChatStateService;
  let mockTransport: { connect: Mock, send: Mock, disconnect: Mock };
  let wsSubject: Subject<WSEvent>;

  beforeEach(() => {
    wsSubject = new Subject<WSEvent>();
    
    mockTransport = {
      connect: vi.fn(() => wsSubject.asObservable()),
      send: vi.fn(),
      disconnect: vi.fn(),
    };

    TestBed.configureTestingModule({
      providers: [
        ChatStateService,
        { provide: TransportService, useValue: mockTransport }
      ]
    });
    service = TestBed.inject(ChatStateService);
  });

  it('should update connection status and initialize on connect', () => {
    service.connect('ws://test');
    expect(mockTransport.connect).toHaveBeenCalledWith('ws://test');
    expect(service.connected()).toBe(true);
  });

  it('should stream model chunks into a single coreChat item', () => {
    service.connect('ws://test');
    
    const chunk1 = create(WSEventSchema, { payload: { case: 'chatStream', value: { text: 'Hello ' } } });
    const chunk2 = create(WSEventSchema, { payload: { case: 'chatStream', value: { text: 'World' } } });
    
    wsSubject.next(chunk1);
    wsSubject.next(chunk2);

    const chat = service.coreChat();
    expect(chat.length).toBe(1);
    expect(chat[0].source).toBe('model');
    expect(chat[0].content).toBe('Hello World');
  });

  it('should trigger pending strategy state on request_strategy', () => {
    service.connect('ws://test');
    
    const reqStrategy = create(WSEventSchema, { payload: { case: 'requestStrategy', value: { active: true } } });
    wsSubject.next(reqStrategy);

    expect(service.pendingStrategy()).toBe(true);
  });

  it('should trigger pending review state on request_review', () => {
    service.connect('ws://test');
    
    const reqReview = create(WSEventSchema, { payload: { case: 'requestReview', value: { branch: 'candidate/123' } } });
    wsSubject.next(reqReview);

    expect(service.pendingReviewBranch()).toBe('candidate/123');
  });

  it('should send prompt and append user message to coreChat', () => {
    service.submitPrompt('Refactor this app');
    
    expect(mockTransport.send).toHaveBeenCalled();
    const chat = service.coreChat();
    expect(chat.length).toBe(1);
    expect(chat[0].source).toBe('user');
    expect(chat[0].content).toBe('Refactor this app');
  });

  it('should clear strategy state and send on selectStrategy', () => {
    service.pendingStrategy.set(true);
    service.selectStrategy(DomainDelegationStrategy.REFINE);
    
    expect(service.pendingStrategy()).toBe(false);
    expect(mockTransport.send).toHaveBeenCalled();
  });

  it('should clear review state and send on submitReview', () => {
    service.pendingReviewBranch.set('candidate/123');
    service.submitReview(true);
    
    expect(service.pendingReviewBranch()).toBeNull();
    expect(mockTransport.send).toHaveBeenCalled();
  });

  it('should route delegation start to both systemLogs and coreChat', () => {
    service.connect('ws://test');
    
    wsSubject.next(create(WSEventSchema, { 
      payload: { case: 'delegationStart', value: { agentCount: 2, instructions: 'Do work' } } 
    }));

    expect(service.systemLogs().length).toBe(1);
    expect(service.systemLogs()[0].message).toContain('Delegating task to 2');
    
    expect(service.coreChat().length).toBe(1);
    expect(service.coreChat()[0].source).toBe('system');
    expect(service.coreChat()[0].content).toContain('Delegating task to 2');
  });

  it('should track agent lifecycle in activeAgents map', () => {
    service.connect('ws://test');
    
    // Start Agent
    wsSubject.next(create(WSEventSchema, { 
      payload: { case: 'agentStart', value: { agentId: 1, instructions: 'Code something' } } 
    }));
    
    expect(service.activeAgents().has(1)).toBe(true);
    expect(service.activeAgents().get(1)?.status).toBe('running');

    // Stream text to Agent
    wsSubject.next(create(WSEventSchema, { 
      payload: { case: 'agentStream', value: { agentId: 1, text: 'func main() {}' } } 
    }));

    expect(service.activeAgents().get(1)?.stream).toBe('func main() {}');

    // Complete Agent
    wsSubject.next(create(WSEventSchema, { 
      payload: { case: 'agentComplete', value: { agentId: 1, branch: 'feat/test', verified: true } } 
    }));

    const finalAgent = service.activeAgents().get(1);
    expect(finalAgent?.status).toBe('completed');
    expect(finalAgent?.branch).toBe('feat/test');
    expect(finalAgent?.verified).toBe(true);
  });
});
```


### `libs\llm\state\chat\src\lib\chat-state.service.ts`
```
import { Injectable, signal, inject } from '@angular/core';
import { TransportService } from '@org/llm-core-transport';
import { LlmFacade, DomainEvent, DomainDelegationStrategy } from '@org/llm-core-facade';

export interface ChatItem {
  id: string;
  source: 'user' | 'model' | 'system';
  content: string;
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

@Injectable({ providedIn: 'root' })
export class ChatStateService {
  private transport = inject(TransportService);

  // Partitioned State Signals
  public coreChat = signal<ChatItem[]>([]);
  public systemLogs = signal<LogItem[]>([]);
  public activeAgents = signal<Map<number, AgentState>>(new Map());
  
  public spaces = signal<{id: string; name: string}[]>([]);
  public activeSpaceId = signal<string>('golang');
  public pendingStrategy = signal<boolean>(false);
  public pendingReviewBranch = signal<string | null>(null);
  public connected = signal<boolean>(false);

  public connect(url: string): void {
    this.transport.connect(url).subscribe({
      next: (proto) => {
        const domainEvent = LlmFacade.toDomain(proto);
        if (domainEvent) {
          this.handleEvent(domainEvent);
        }
      },
      error: (err) => {
        this.appendSystemLog('ERROR', `Connection error: ${err}`);
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
    this.transport.disconnect();
    this.connected.set(false);
  }

  public submitPrompt(text: string): void {
    this.appendCoreChat('user', text);
    const proto = LlmFacade.createSubmitPrompt(text, this.activeSpaceId());
    this.transport.send(proto);
  }

  private handleEvent(event: DomainEvent): void {
    switch (event.type) {
      case 'available_spaces':
        this.spaces.set(event.spaces);
        this.appendSystemLog('INFO', `Available spaces: ${event.spaces.map(s => s.id).join(', ')}`);
        break;
      
      case 'chat_stream':
        this.appendToLastModelMessage(event.text);
        break;
      
      case 'log_message':
        this.appendSystemLog(event.level, event.message);
        break;
      
      case 'delegation_start':
        // Log to system, but ALSO drop a contextual note in the main chat feed
        const msg = `🚀 Delegating task to ${event.agentCount} agent(s): ${event.instructions}`;
        this.appendSystemLog('INFO', msg);
        this.appendCoreChat('system', msg);
        break;
      
      case 'delegation_complete':
        this.appendSystemLog('INFO', `✅ Delegation Flow Complete:\n${event.summary}`);
        break;
      
      case 'agent_start':
        this.activeAgents.update(map => {
          const newMap = new Map(map);
          newMap.set(event.agentId, {
            id: event.agentId,
            instructions: event.instructions,
            stream: '',
            status: 'running'
          });
          return newMap;
        });
        break;
      
      case 'agent_stream':
        this.activeAgents.update(map => {
          const newMap = new Map(map);
          const agent = newMap.get(event.agentId);
          if (agent) {
            agent.stream += event.text;
            newMap.set(event.agentId, agent);
          }
          return newMap;
        });
        break;
      
      case 'agent_complete':
        this.activeAgents.update(map => {
          const newMap = new Map(map);
          const agent = newMap.get(event.agentId);
          if (agent) {
            agent.status = 'completed';
            agent.branch = event.branch;
            agent.verified = event.verified;
            newMap.set(event.agentId, agent);
          }
          return newMap;
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

  private appendCoreChat(source: ChatItem['source'], content: string): void {
    this.coreChat.update(chat => [...chat, { id: crypto.randomUUID(), source, content }]);
  }

  private appendToLastModelMessage(chunk: string): void {
    this.coreChat.update(chat => {
      if (chat.length === 0 || chat[chat.length - 1].source !== 'model') {
        return [...chat, { id: crypto.randomUUID(), source: 'model', content: chunk }];
      }
      const newChat = [...chat];
      newChat[newChat.length - 1].content += chunk;
      return newChat;
    });
  }

  private appendSystemLog(level: string, message: string): void {
    this.systemLogs.update(logs => [...logs, { 
      id: crypto.randomUUID(), 
      timestamp: Date.now(),
      level, 
      message 
    }]);
  }

  public selectStrategy(strategy: DomainDelegationStrategy): void {
    this.pendingStrategy.set(false);
    // Explicitly record the user's decision in the main chat feed
    this.appendCoreChat('user', `Selected Strategy: ${DomainDelegationStrategy[strategy]}`);
    
    const proto = LlmFacade.createSelectStrategy(strategy);
    this.transport.send(proto);
  }

  public submitReview(accepted: boolean): void {
    const branch = this.pendingReviewBranch();
    if (!branch) return;
    
    this.pendingReviewBranch.set(null);
    // Explicitly record the user's branch decision in the main chat feed
    this.appendCoreChat('user', `Review for ${branch}: ${accepted ? 'ACCEPTED' : 'REJECTED'}`);
    
    const proto = LlmFacade.createReviewDecision(branch, accepted);
    this.transport.send(proto);
  }
}
```
