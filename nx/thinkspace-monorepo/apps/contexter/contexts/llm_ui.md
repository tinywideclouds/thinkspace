

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
import { nxViteTsPaths } from '@nx/vite/plugins/nx-tsconfig-paths.plugin';
import { nxCopyAssetsPlugin } from '@nx/vite/plugins/nx-copy-assets.plugin';

export default defineConfig(() => ({
  root: __dirname,
  cacheDir: '../../../../node_modules/.vite/libs/llm/ui/chat',
  plugins: [angular(), nxViteTsPaths(), nxCopyAssetsPlugin(['*.md'])],
  // Uncomment this if you are using workers.
  // worker: {
  //   plugins: () => [ nxViteTsPaths() ],
  // },
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
import { Component, input } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ChatItem } from '@org/llm-state-chat';

@Component({
  selector: 'llm-chat-feed',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div style="display: flex; flex-direction: column; gap: 8px;">
      @for (item of feed(); track item.id) {
        <div style="padding: 8px; border-radius: 4px; background: #f0f0f0;">
          <strong style="text-transform: uppercase; font-size: 0.8em; color: #555;">{{ item.source }}</strong>
          <pre style="margin: 4px 0 0; white-space: pre-wrap; font-family: monospace;">{{ item.content }}</pre>
        </div>
      }
    </div>
  `
})
export class ChatFeedComponent {
  feed = input.required<ChatItem[]>();
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
import { Component, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';

@Component({
  selector: 'llm-chat-input',
  standalone: true,
  imports: [FormsModule],
  template: `
    <div style="display: flex; gap: 8px; margin-top: 16px;">
      <input 
        type="text" 
        [(ngModel)]="promptText" 
        (keyup.enter)="submit()"
        placeholder="Type your message..." 
        style="flex: 1; padding: 8px; border: 1px solid #ccc; border-radius: 4px;"
      />
      <button (click)="submit()" style="padding: 8px 16px; cursor: pointer;">Send</button>
    </div>
  `
})
export class ChatInputComponent {
  promptText = signal('');
  sendPrompt = output<string>();

  submit() {
    const text = this.promptText().trim();
    if (text) {
      this.sendPrompt.emit(text);
      this.promptText.set('');
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
