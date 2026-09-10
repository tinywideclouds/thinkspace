

### `libs\llm\feature\chat\eslint.config.mjs`
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


### `libs\llm\feature\chat\project.json`
```
{
  "name": "llm-feature-chat",
  "$schema": "../../../../node_modules/nx/schemas/project-schema.json",
  "sourceRoot": "libs/llm/feature/chat/src",
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


### `libs\llm\feature\chat\README.md`
```
# llm-feature-chat

This library was generated with [Nx](https://nx.dev).

## Running unit tests

Run `nx test llm-feature-chat` to execute the unit tests.

```


### `libs\llm\feature\chat\tsconfig.json`
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


### `libs\llm\feature\chat\tsconfig.lib.json`
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


### `libs\llm\feature\chat\tsconfig.spec.json`
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


### `libs\llm\feature\chat\vite.config.mts`
```
/// <reference types='vitest' />
import { defineConfig } from 'vite';
import angular from '@analogjs/vite-plugin-angular';

export default defineConfig(() => ({
  root: __dirname,
  cacheDir: '../../../../node_modules/.vite/libs/llm/feature/chat',
  resolve: {
    tsconfigPaths: true,
  },
  plugins: [angular()],
  test: {
    name: 'llm-feature-chat',
    watch: false,
    globals: true,
    environment: 'jsdom',
    include: ['{src,tests}/**/*.{test,spec}.{js,mjs,cjs,ts,mts,cts,jsx,tsx}'],
    setupFiles: ['src/test-setup.ts'],
    reporters: ['default'],
    coverage: {
      reportsDirectory: '../../../../coverage/libs/llm/feature/chat',
      provider: 'v8' as const,
    },
  },
}));
```


### `libs\llm\feature\chat\src\index.ts`
```
export * from './lib/chat-container/chat-container.component';
```


### `libs\llm\feature\chat\src\test-setup.ts`
```
import '@angular/compiler';
import '@analogjs/vitest-angular/setup-snapshots';
import { setupTestBed } from '@analogjs/vitest-angular/setup-testbed';

setupTestBed();

```


### `libs\llm\feature\chat\src\lib\chat-container\chat-container.component.css`
```
.chat-layout {
  display: flex;
  flex-direction: column;
  height: 100vh;
  width: 100vw;
  overflow: hidden;
  background-color: #f8f9fa;
  font-family: system-ui, -apple-system, sans-serif;
}

.chat-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  max-width: 900px;
  margin: 0 auto;
  width: 100%;
  background: white;
  border-left: 1px solid #dee2e6;
  border-right: 1px solid #dee2e6;
  box-shadow: 0 0 15px rgba(0,0,0,0.05);
  overflow: hidden;
}

.chat-header {
  padding: 16px 24px;
  border-bottom: 1px solid #dee2e6;
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: #fff;
  z-index: 10;
}

.chat-header h2 {
  margin: 0;
  font-size: 18px;
  color: #212529;
}

.connection-status {
  font-size: 12px;
  font-weight: bold;
  padding: 4px 8px;
  border-radius: 12px;
  background: #ffe3e3;
  color: #c92a2a;
}

.connection-status.connected {
  background: #ebfbee;
  color: #2b8a3e;
}

.chat-scroll-area {
  flex: 1;
  overflow-y: auto;
  padding: 24px;
  display: flex;
  flex-direction: column;
}

.prompt-area {
  margin-top: auto; /* Pushes input to bottom */
  padding-top: 24px;
}

.chat-drawer {
  height: 250px;
  border-top: 2px solid #dee2e6;
  background: white;
  flex-shrink: 0;
  z-index: 20;
}
```


### `libs\llm\feature\chat\src\lib\chat-container\chat-container.component.html`
```
<div class="chat-layout">
  
  <main class="chat-main">
    <div class="chat-header">
      <!-- Replaced static H2 with dynamic header -->
      <llm-chat-header
        style="flex: 1; margin-right: 24px;"
        [spaces]="workspaceState.spaces()"
        [activeSpaceId]="workspaceState.activeSpaceId()"
        [chats]="workspaceState.chats()"
        [activeChatId]="workspaceState.activeChatId()"
        (spaceSelected)="onSpaceSelected($event)"
        (chatSelected)="onChatSelected($event)"
        (newChatRequested)="onNewChatRequested()">
      </llm-chat-header>
      
      <div class="connection-status" [class.connected]="chatState.connected()">
        {{ chatState.connected() ? '🟢 Connected' : '🔴 Disconnected' }}
      </div>
    </div>
    
    <div class="chat-scroll-area">
      <llm-chat-feed 
        [feed]="chatState.coreChat()" 
        (inspectFlow)="chatState.inspectFlow($event)"
      />

      <div class="prompt-area">
        @if (chatState.pendingStrategy()) {
          <llm-chat-strategy-prompt 
            (strategySelected)="chatState.selectStrategy($event)" 
          />
        } @else if (chatState.pendingReviewBranch(); as branch) {
          <llm-chat-review-prompt 
            [branch]="branch" 
            (reviewDecided)="chatState.submitReview($event)" 
          />
        } @else {
          <llm-chat-input 
            (sendPrompt)="chatState.submitPrompt($event)" 
          />
        }
      </div>
    </div>
  </main>

  <aside class="chat-drawer">
    <llm-flow-tracker [flows]="chatState.flowStates()"></llm-flow-tracker>
  </aside>

  @if (chatState.inspectedReceipt()) {
    <llm-flow-inspector 
      [receiptXml]="chatState.inspectedReceipt()!"
      (close)="chatState.inspectedReceipt.set(null)">
    </llm-flow-inspector>
  }
</div>
```


### `libs\llm\feature\chat\src\lib\chat-container\chat-container.component.ts`
```
import { Component, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ChatStateService, WorkspaceStateService } from '@org/llm-state-chat';
import { 
  ChatFeedComponent, 
  ChatInputComponent, 
  ChatStrategyPromptComponent, 
  ChatReviewPromptComponent,
  FlowTrackerComponent,
  FlowInspectorComponent,
  ChatHeaderComponent
} from '@org/llm-ui-chat';

@Component({
  selector: 'llm-chat-container',
  standalone: true,
  imports: [
    CommonModule,
    ChatFeedComponent,
    ChatInputComponent,
    ChatStrategyPromptComponent,
    ChatReviewPromptComponent,
    FlowTrackerComponent,
    FlowInspectorComponent,
    ChatHeaderComponent
  ],
  templateUrl: './chat-container.component.html',
  styleUrl: './chat-container.component.css'
})
export class ChatContainerComponent {
  public chatState = inject(ChatStateService);
  public workspaceState = inject(WorkspaceStateService);

  onSpaceSelected(spaceId: string) {
    this.workspaceState.loadChats(spaceId);
    this.chatState.clearSession();
  }

  onChatSelected(chatId: string) {
    this.workspaceState.activeChatId.set(chatId);
    this.chatState.clearSession();
  }

  onNewChatRequested() {
    const name = prompt('Enter a name for the new chat (or leave blank for auto):');
    if (name !== null) { // null means the user cancelled the prompt
      this.workspaceState.createChat(name);
      this.chatState.clearSession();
    }
  }
}
```
