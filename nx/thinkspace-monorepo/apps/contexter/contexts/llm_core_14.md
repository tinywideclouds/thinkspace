

### `libs/llm/core/facade/eslint.config.mjs`
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


### `libs/llm/core/facade/project.json`
```
{
  "name": "llm-core-facade",
  "$schema": "../../../../node_modules/nx/schemas/project-schema.json",
  "sourceRoot": "libs/llm/core/facade/src",
  "prefix": "lib",
  "projectType": "library",
  "tags": ["scope:llm", "type:facade"],
  "targets": {
    "lint": {
      "executor": "@nx/eslint:lint"
    }
  }
}

```


### `libs/llm/core/facade/README.md`
```
# llm-core-facade

This library acts as an Anti-Corruption Layer (ACL) / Facade between the network transport (Protobuf) and the frontend application domain.

## Responsibilities
* Define the core TypeScript Domain Interfaces (`DomainEvent`, `DomainDelegationStrategy`, etc.).
* Translate incoming Protobuf `WSEvent` schemas into clean, decoupled Domain types.
* Translate outgoing Domain actions into Protobuf `WSEvent` objects ready for the transport layer.

## Rules
* **MUST** consist of pure, stateless functions.
* **MUST** be the *only* library in the workspace (outside of transport) that imports from `@org/llm-core-protos`. 
* The UI and State layers must never know Protobuf exists.
```


### `libs/llm/core/facade/tsconfig.json`
```
{
  "extends": "../../../../tsconfig.base.json",
  "compilerOptions": {
    "isolatedModules": true,
    "target": "esnext",
    "strict": true,
    "noImplicitOverride": true,
    "noPropertyAccessFromIndexSignature": true,
    "noImplicitReturns": true,
    "noFallthroughCasesInSwitch": true,
    "emitDecoratorMetadata": false,
    "forceConsistentCasingInFileNames": true,
    "module": "esnext"
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


### `libs/llm/core/facade/tsconfig.lib.json`
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


### `libs/llm/core/facade/tsconfig.spec.json`
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


### `libs/llm/core/facade/vite.config.mts`
```
/// <reference types='vitest' />
import { defineConfig } from 'vite';
import angular from '@analogjs/vite-plugin-angular';

export default defineConfig(() => ({
  root: __dirname,
  cacheDir: '../../../../node_modules/.vite/libs/llm/core/facade',
  resolve: {
    tsconfigPaths: true,
  },
  plugins: [angular()],
  test: {
    name: 'llm-core-facade',
    watch: false,
    globals: true,
    environment: 'jsdom',
    include: ['{src,tests}/**/*.{test,spec}.{js,mjs,cjs,ts,mts,cts,jsx,tsx}'],
    setupFiles: ['src/test-setup.ts'],
    reporters: ['default'],
    coverage: {
      reportsDirectory: '../../../../coverage/libs/llm/core/facade',
      provider: 'v8' as const,
    },
  },
}));
```


### `libs/llm/core/facade/src/index.ts`
```
export * from './lib/domain-models';
export * from './lib/facade';
```


### `libs/llm/core/facade/src/test-setup.ts`
```
import '@angular/compiler';
import '@analogjs/vitest-angular/setup-snapshots';
import { setupTestBed } from '@analogjs/vitest-angular/setup-testbed';

setupTestBed({ zoneless: false });

```


### `libs/llm/core/facade/src/lib/domain-models.ts`
```
export interface DomainFlowEvent {
  type: 'flow_event';
  flowId: string;
  eventType: string;
  timestamp: string;
  taskId: string;
  agentCount: number;
  agentId: string;
  agentIndex: number;
  instruction: string;
  status: string;
  attempt: number;
  trace: string;
  candidateId: string;
  passed: boolean;
}

export interface SpaceInfo {
  id: string;
  name: string;
}

export interface DomainLedgerEvent {
  id: string;
  timestamp: string;
  type: string;
  content: string;
  metadata: Record<string, string>;
}

export interface DomainDigestMeta {
  id: string;
  summary: string;
  isSticky: boolean;
}

export type DomainEvent =
  | { type: 'chat_stream'; text: string }
  | { type: 'log_message'; level: string; message: string }
  | { type: 'delegation_start'; agentCount: number; instructions: string }
  | { type: 'delegation_complete'; summary: string }
  | { type: 'agent_start'; agentId: number; instructions: string }
  | { type: 'agent_stream'; agentId: number; text: string }
  | { type: 'agent_complete'; agentId: number; branch: string; verified: boolean }
  | { type: 'request_strategy'; active: boolean }
  | { type: 'request_review'; branch: string }
  | { type: 'available_spaces'; spaces: SpaceInfo[] }
  | { type: 'sync_history'; recentEvents: DomainLedgerEvent[]; digests: Record<string, DomainDigestMeta> }
  | DomainFlowEvent;

export enum DomainDelegationStrategy {
  SKIP = 1,
  MANUAL = 2,
  REVIEW = 3,
  REFINE = 4,
}
```


### `libs/llm/core/facade/src/lib/facade.spec.ts`
```
import { describe, it, expect } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { WSEventSchema, DelegationStrategy } from '@org/llm-core-protos';
import { LlmFacade } from './facade';
import { DomainDelegationStrategy } from './domain-models';

describe('LlmFacade', () => {
  describe('toDomain (Inbound)', () => {
    it('should map a chat stream event correctly', () => {
      const protocolBufferEvent = create(WSEventSchema, {
        payload: {
          case: 'chatStream',
          value: { text: 'Hello from model' }
        }
      });
      const domainEvent = LlmFacade.toDomain(protocolBufferEvent);
      expect(domainEvent).toEqual({ type: 'chat_stream', text: 'Hello from model' });
    });

    it('should map an available spaces event correctly', () => {
      const protocolBufferEvent = create(WSEventSchema, {
        payload: {
          case: 'availableSpaces',
          value: {
            spaces: [
              { id: 'golang', name: 'Go Developer' },
              { id: 'angular', name: 'Angular Expert' }
            ]
          }
        }
      });

      const domainEvent = LlmFacade.toDomain(protocolBufferEvent);
      expect(domainEvent).toEqual({
        type: 'available_spaces',
        spaces: [
          { id: 'golang', name: 'Go Developer' },
          { id: 'angular', name: 'Angular Expert' }
        ]
      });
    });

    it('should map a sync history event correctly', () => {
      const protocolBufferEvent = create(WSEventSchema, {
        payload: {
          case: 'syncHistory',
          value: {
            recentEvents: [
              { id: '1', timestamp: '2026-09-02T15:00:00Z', type: 'prompt', content: 'test', metadata: { key: 'val' } }
            ],
            digests: {
              'abc': { id: 'abc', summary: 'test digest', isSticky: true }
            }
          }
        }
      });

      const domainEvent = LlmFacade.toDomain(protocolBufferEvent);
      expect(domainEvent).toEqual({
        type: 'sync_history',
        recentEvents: [
          { id: '1', timestamp: '2026-09-02T15:00:00Z', type: 'prompt', content: 'test', metadata: { key: 'val' } }
        ],
        digests: {
          'abc': { id: 'abc', summary: 'test digest', isSticky: true }
        }
      });
    });

    it('should map a flow event correctly', () => {
      const protocolBufferEvent = create(WSEventSchema, {
        payload: {
          case: 'flowEvent',
          value: {
            flowId: 'flow-123',
            type: 'flow_status',
            timestamp: '2026-09-02T15:00:00Z',
            taskId: 'task-1',
            agentCount: 2,
            agentId: 'agent-1',
            agentIndex: 1,
            instruction: 'Do work',
            status: 'running_tests',
            attempt: 2,
            trace: 'compile error',
            candidateId: 'cand-1',
            passed: false
          }
        }
      });

      const domainEvent = LlmFacade.toDomain(protocolBufferEvent);
      expect(domainEvent).toEqual({
        type: 'flow_event',
        flowId: 'flow-123',
        eventType: 'flow_status',
        timestamp: '2026-09-02T15:00:00Z',
        taskId: 'task-1',
        agentCount: 2,
        agentId: 'agent-1',
        agentIndex: 1,
        instruction: 'Do work',
        status: 'running_tests',
        attempt: 2,
        trace: 'compile error',
        candidateId: 'cand-1',
        passed: false
      });
    });

    it('should return null for undefined payload or case', () => {
      const emptyProtocolBufferEvent = create(WSEventSchema);
      expect(LlmFacade.toDomain(emptyProtocolBufferEvent)).toBeNull();
    });
  });

  describe('Outbound Builders', () => {
    it('should create a valid SubmitPrompt WSEvent', () => {
      const protocolBufferEvent = LlmFacade.createSubmitPrompt('Write a test', 'golang', 'test-chat');
      
      expect(protocolBufferEvent.payload.case).toBe('submitPrompt');
      if (protocolBufferEvent.payload.case === 'submitPrompt') {
        expect(protocolBufferEvent.payload.value.text).toBe('Write a test');
        expect(protocolBufferEvent.payload.value.spaceId).toBe('golang');
        expect(protocolBufferEvent.payload.value.chatId).toBe('test-chat');
      }
    });

    it('should create a valid SelectStrategy WSEvent', () => {
      const protocolBufferEvent = LlmFacade.createSelectStrategy(DomainDelegationStrategy.REFINE);
      
      expect(protocolBufferEvent.payload.case).toBe('selectStrategy');
      if (protocolBufferEvent.payload.case === 'selectStrategy') {
        expect(protocolBufferEvent.payload.value.strategyId).toBe(DelegationStrategy.REFINE);
      }
    });

    it('should create a valid ReviewDecision WSEvent', () => {
      const protocolBufferEvent = LlmFacade.createReviewDecision('candidate/123', true);
      
      expect(protocolBufferEvent.payload.case).toBe('reviewDecision');
      if (protocolBufferEvent.payload.case === 'reviewDecision') {
        expect(protocolBufferEvent.payload.value.branch).toBe('candidate/123');
        expect(protocolBufferEvent.payload.value.accepted).toBe(true);
      }
    });
  });
});
```


### `libs/llm/core/facade/src/lib/facade.ts`
```
import { create } from '@bufbuild/protobuf';
import { 
  WSEvent, 
  WSEventSchema,
  DelegationStrategy 
} from '@org/llm-core-protos';
import { DomainEvent, DomainDelegationStrategy, DomainDigestMeta } from './domain-models';

export class LlmFacade {
  static toDomain(protocolBufferEvent: WSEvent): DomainEvent | null {
    if (!protocolBufferEvent.payload || protocolBufferEvent.payload.case === undefined) {
      return null;
    }

    switch (protocolBufferEvent.payload.case) {
      case 'chatStream':
        return { type: 'chat_stream', text: protocolBufferEvent.payload.value.text };
      
      case 'logMessage':
        return { type: 'log_message', level: protocolBufferEvent.payload.value.level, message: protocolBufferEvent.payload.value.message };
      
      case 'agentStream':
        return { type: 'agent_stream', agentId: protocolBufferEvent.payload.value.agentId, text: protocolBufferEvent.payload.value.text };
      
      case 'requestStrategy':
        return { type: 'request_strategy', active: protocolBufferEvent.payload.value.active };
      
      case 'requestReview':
        return { type: 'request_review', branch: protocolBufferEvent.payload.value.branch };
      
      case 'availableSpaces':
        return { type: 'available_spaces', spaces: protocolBufferEvent.payload.value.spaces.map(space => ({ id: space.id, name: space.name })) };
      
      case 'syncHistory':
        return {
          type: 'sync_history',
          recentEvents: protocolBufferEvent.payload.value.recentEvents.map(e => ({
            id: e.id,
            timestamp: e.timestamp,
            type: e.type,
            content: e.content,
            metadata: e.metadata
          })),
          digests: Object.entries(protocolBufferEvent.payload.value.digests).reduce((acc, [key, val]) => {
            acc[key] = { id: val.id, summary: val.summary, isSticky: val.isSticky };
            return acc;
          }, {} as Record<string, DomainDigestMeta>)
        };

      case 'flowEvent':
        return {
          type: 'flow_event',
          flowId: protocolBufferEvent.payload.value.flowId,
          eventType: protocolBufferEvent.payload.value.type,
          timestamp: protocolBufferEvent.payload.value.timestamp,
          taskId: protocolBufferEvent.payload.value.taskId,
          agentCount: protocolBufferEvent.payload.value.agentCount,
          agentId: protocolBufferEvent.payload.value.agentId,
          agentIndex: protocolBufferEvent.payload.value.agentIndex,
          instruction: protocolBufferEvent.payload.value.instruction,
          status: protocolBufferEvent.payload.value.status,
          attempt: protocolBufferEvent.payload.value.attempt,
          trace: protocolBufferEvent.payload.value.trace,
          candidateId: protocolBufferEvent.payload.value.candidateId,
          passed: protocolBufferEvent.payload.value.passed
        };
      
      default:
        console.warn(`[LlmFacade] Unhandled inbound protocol buffer case: ${protocolBufferEvent.payload.case}`);
        return null;
    }
  }

  static createSubmitPrompt(text: string, spaceId: string, chatId: string): WSEvent {
    return create(WSEventSchema, {
      payload: {
        case: 'submitPrompt',
        value: { text, spaceId, chatId }
      }
    });
  }

  static createSelectStrategy(strategy: DomainDelegationStrategy): WSEvent {
    const strategyId = strategy as unknown as DelegationStrategy;
    return create(WSEventSchema, {
      payload: {
        case: 'selectStrategy',
        value: { strategyId }
      }
    });
  }

  static createReviewDecision(branch: string, accepted: boolean): WSEvent {
    return create(WSEventSchema, {
      payload: {
        case: 'reviewDecision',
        value: { branch, accepted }
      }
    });
  }
}
```


### `libs/llm/core/protos/buf.gen.yaml`
```
version: v1
plugins:
  - plugin: es
    opt:
      - target=ts
    out: src/generated
  - plugin: go
    opt:
      - module=github.com/tinywideclouds/thinkspace/api/v1
    out: ../../../../../../go/api/v1
```


### `libs/llm/core/protos/eslint.config.mjs`
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


### `libs/llm/core/protos/project.json`
```
{
  "name": "llm-core-protos",
  "$schema": "../../../../node_modules/nx/schemas/project-schema.json",
  "sourceRoot": "libs/llm/core/protos/src",
  "prefix": "lib",
  "projectType": "library",
  "tags": ["scope:llm", "type:protos"],
  "targets": {
    "lint": {
      "executor": "@nx/eslint:lint"
    }
  }
}

```


### `libs/llm/core/protos/README.md`
```
# llm-core-protos

This library contains the generated TypeScript bindings for the ThinkSpace Protobuf definitions. It serves as the strict network boundary contract between the Golang orchestration backend and the Angular frontend.

## Responsibilities
* House the generated `@bufbuild/protobuf` schemas (`events.proto`).
* Provide strongly-typed `WSEvent` wrappers and payload structures.
* Ensure type safety for JSON serialization/deserialization over WebSockets.

## Rules
* **DO NOT** write manual business logic in this library.
* **DO NOT** modify the generated `.ts` files manually. Always use the `buf generate` pipeline to update the schemas.
```


### `libs/llm/core/protos/tsconfig.json`
```
{
  "extends": "../../../../tsconfig.base.json",
  "compilerOptions": {
    "isolatedModules": true,
    "target": "esnext",
    "strict": true,
    "noImplicitOverride": true,
    "noPropertyAccessFromIndexSignature": true,
    "noImplicitReturns": true,
    "noFallthroughCasesInSwitch": true,
    "emitDecoratorMetadata": false,
    "forceConsistentCasingInFileNames": true,
    "module": "esnext"
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


### `libs/llm/core/protos/tsconfig.lib.json`
```
{
  "extends": "./tsconfig.json",
  "compilerOptions": {
    "outDir": "../../../../dist/out-tsc",
    "declaration": true,
    "declarationMap": true,
    "inlineSources": true,
    "forceConsistentCasingInFileNames": true,
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


### `libs/llm/core/protos/tsconfig.spec.json`
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


### `libs/llm/core/protos/vite.config.mts`
```
/// <reference types='vitest' />
import { defineConfig } from 'vite';
import angular from '@analogjs/vite-plugin-angular';
import { nxViteTsPaths } from '@nx/vite/plugins/nx-tsconfig-paths.plugin';
import { nxCopyAssetsPlugin } from '@nx/vite/plugins/nx-copy-assets.plugin';

export default defineConfig(() => ({
  root: __dirname,
  cacheDir: '../../../../node_modules/.vite/libs/llm/core/protos',
  plugins: [angular(), nxViteTsPaths(), nxCopyAssetsPlugin(['*.md'])],
  // Uncomment this if you are using workers.
  // worker: {
  //   plugins: () => [ nxViteTsPaths() ],
  // },
  test: {
    name: 'llm-core-protos',
    watch: false,
    globals: true,
    environment: 'jsdom',
    include: ['{src,tests}/**/*.{test,spec}.{js,mjs,cjs,ts,mts,cts,jsx,tsx}'],
    setupFiles: ['src/test-setup.ts'],
    reporters: ['default'],
    coverage: {
      reportsDirectory: '../../../../coverage/libs/llm/core/protos',
      provider: 'v8' as const,
    },
  },
}));

```


### `libs/llm/core/protos/src/index.ts`
```
export * from './generated/src/thinkspace/api/v1/events_pb';

```


### `libs/llm/core/protos/src/test-setup.ts`
```
import '@angular/compiler';
import '@analogjs/vitest-angular/setup-snapshots';
import { setupTestBed } from '@analogjs/vitest-angular/setup-testbed';

setupTestBed({ zoneless: false });

```


### `libs/llm/core/protos/src/generated/src/thinkspace/api/v1/events_pb.ts`
```
// @generated by protoc-gen-es v2.14.0 with parameter "target=ts"
// @generated from file src/thinkspace/api/v1/events.proto (package thinkspace.api.v1, syntax proto3)
/* eslint-disable */

import type { GenEnum, GenFile, GenMessage } from "@bufbuild/protobuf/codegenv2";
import { enumDesc, fileDesc, messageDesc } from "@bufbuild/protobuf/codegenv2";
import type { Message } from "@bufbuild/protobuf";

/**
 * Describes the file src/thinkspace/api/v1/events.proto.
 */
export const file_src_thinkspace_api_v1_events: GenFile = /*@__PURE__*/
  fileDesc("CiJzcmMvdGhpbmtzcGFjZS9hcGkvdjEvZXZlbnRzLnByb3RvEhF0aGlua3NwYWNlLmFwaS52MSI8CglTcGFjZUluZm8SCgoCaWQYASABKAkSDAoEbmFtZRgCIAEoCRIVCg1pc19jb25maWd1cmVkGAMgASgIIrwBCgtMZWRnZXJFdmVudBIKCgJpZBgBIAEoCRIRCgl0aW1lc3RhbXAYAiABKAkSDAoEdHlwZRgDIAEoCRIPCgdjb250ZW50GAQgASgJEj4KCG1ldGFkYXRhGAUgAygLMiwudGhpbmtzcGFjZS5hcGkudjEuTGVkZ2VyRXZlbnQuTWV0YWRhdGFFbnRyeRovCg1NZXRhZGF0YUVudHJ5EgsKA2tleRgBIAEoCRINCgV2YWx1ZRgCIAEoCToCOAEiPAoKRGlnZXN0TWV0YRIKCgJpZBgBIAEoCRIPCgdzdW1tYXJ5GAIgASgJEhEKCWlzX3N0aWNreRgDIAEoCCIhChFDaGF0U3RyZWFtUGF5bG9hZBIMCgR0ZXh0GAEgASgJIjMKEUxvZ01lc3NhZ2VQYXlsb2FkEg0KBWxldmVsGAEgASgJEg8KB21lc3NhZ2UYAiABKAkiNAoSQWdlbnRTdHJlYW1QYXlsb2FkEhAKCGFnZW50X2lkGAEgASgFEgwKBHRleHQYAiABKAki3wEKElN5bmNIaXN0b3J5UGF5bG9hZBI1Cg1yZWNlbnRfZXZlbnRzGAEgAygLMh4udGhpbmtzcGFjZS5hcGkudjEuTGVkZ2VyRXZlbnQSQwoHZGlnZXN0cxgCIAMoCzIyLnRoaW5rc3BhY2UuYXBpLnYxLlN5bmNIaXN0b3J5UGF5bG9hZC5EaWdlc3RzRW50cnkaTQoMRGlnZXN0c0VudHJ5EgsKA2tleRgBIAEoCRIsCgV2YWx1ZRgCIAEoCzIdLnRoaW5rc3BhY2UuYXBpLnYxLkRpZ2VzdE1ldGE6AjgBIigKFlJlcXVlc3RTdHJhdGVneVBheWxvYWQSDgoGYWN0aXZlGAEgASgIIiYKFFJlcXVlc3RSZXZpZXdQYXlsb2FkEg4KBmJyYW5jaBgBIAEoCSJGChZBdmFpbGFibGVTcGFjZXNQYXlsb2FkEiwKBnNwYWNlcxgBIAMoCzIcLnRoaW5rc3BhY2UuYXBpLnYxLlNwYWNlSW5mbyL8AQoQRmxvd0V2ZW50UGF5bG9hZBIPCgdmbG93X2lkGAEgASgJEgwKBHR5cGUYAiABKAkSEQoJdGltZXN0YW1wGAMgASgJEg8KB3Rhc2tfaWQYBCABKAkSEwoLYWdlbnRfY291bnQYBSABKAUSEAoIYWdlbnRfaWQYBiABKAkSEwoLYWdlbnRfaW5kZXgYByABKAUSEwoLaW5zdHJ1Y3Rpb24YCCABKAkSDgoGc3RhdHVzGAkgASgJEg8KB2F0dGVtcHQYCiABKAUSDQoFdHJhY2UYCyABKAkSFAoMY2FuZGlkYXRlX2lkGAwgASgJEg4KBnBhc3NlZBgNIAEoCCJGChNTdWJtaXRQcm9tcHRQYXlsb2FkEgwKBHRleHQYASABKAkSEAoIc3BhY2VfaWQYAiABKAkSDwoHY2hhdF9pZBgDIAEoCSJTChVTZWxlY3RTdHJhdGVneVBheWxvYWQSOgoLc3RyYXRlZ3lfaWQYASABKA4yJS50aGlua3NwYWNlLmFwaS52MS5EZWxlZ2F0aW9uU3RyYXRlZ3kiOQoVUmV2aWV3RGVjaXNpb25QYXlsb2FkEg4KBmJyYW5jaBgBIAEoCRIQCghhY2NlcHRlZBgCIAEoCCLjBQoHV1NFdmVudBI7CgtjaGF0X3N0cmVhbRgBIAEoCzIkLnRoaW5rc3BhY2UuYXBpLnYxLkNoYXRTdHJlYW1QYXlsb2FkSAASOwoLbG9nX21lc3NhZ2UYAiABKAsyJC50aGlua3NwYWNlLmFwaS52MS5Mb2dNZXNzYWdlUGF5bG9hZEgAEj0KDGFnZW50X3N0cmVhbRgGIAEoCzIlLnRoaW5rc3BhY2UuYXBpLnYxLkFnZW50U3RyZWFtUGF5bG9hZEgAEkUKEHJlcXVlc3Rfc3RyYXRlZ3kYCCABKAsyKS50aGlua3NwYWNlLmFwaS52MS5SZXF1ZXN0U3RyYXRlZ3lQYXlsb2FkSAASQQoOcmVxdWVzdF9yZXZpZXcYCSABKAsyJy50aGlua3NwYWNlLmFwaS52MS5SZXF1ZXN0UmV2aWV3UGF5bG9hZEgAEkUKEGF2YWlsYWJsZV9zcGFjZXMYCiABKAsyKS50aGlua3NwYWNlLmFwaS52MS5BdmFpbGFibGVTcGFjZXNQYXlsb2FkSAASPQoMc3luY19oaXN0b3J5GA8gASgLMiUudGhpbmtzcGFjZS5hcGkudjEuU3luY0hpc3RvcnlQYXlsb2FkSAASOQoKZmxvd19ldmVudBgOIAEoCzIjLnRoaW5rc3BhY2UuYXBpLnYxLkZsb3dFdmVudFBheWxvYWRIABI/Cg1zdWJtaXRfcHJvbXB0GAsgASgLMiYudGhpbmtzcGFjZS5hcGkudjEuU3VibWl0UHJvbXB0UGF5bG9hZEgAEkMKD3NlbGVjdF9zdHJhdGVneRgMIAEoCzIoLnRoaW5rc3BhY2UuYXBpLnYxLlNlbGVjdFN0cmF0ZWd5UGF5bG9hZEgAEkMKD3Jldmlld19kZWNpc2lvbhgNIAEoCzIoLnRoaW5rc3BhY2UuYXBpLnYxLlJldmlld0RlY2lzaW9uUGF5bG9hZEgAQgkKB3BheWxvYWQqtwEKEkRlbGVnYXRpb25TdHJhdGVneRIjCh9ERUxFR0FUSU9OX1NUUkFURUdZX1VOU1BFQ0lGSUVEEAASHAoYREVMRUdBVElPTl9TVFJBVEVHWV9TS0lQEAESHgoaREVMRUdBVElPTl9TVFJBVEVHWV9NQU5VQUwQAhIeChpERUxFR0FUSU9OX1NUUkFURUdZX1JFVklFVxADEh4KGkRFTEVHQVRJT05fU1RSQVRFR1lfUkVGSU5FEARCNFoyZ2l0aHViLmNvbS90aW55d2lkZWNsb3Vkcy90aGlua3NwYWNlL2FwaS92MTthcGlfdjFiBnByb3RvMw");

/**
 * --- Shared Types ---
 *
 * @generated from message thinkspace.api.v1.SpaceInfo
 */
export type SpaceInfo = Message<"thinkspace.api.v1.SpaceInfo"> & {
  /**
   * @generated from field: string id = 1;
   */
  id: string;

  /**
   * @generated from field: string name = 2;
   */
  name: string;

  /**
   * @generated from field: bool is_configured = 3;
   */
  isConfigured: boolean;
};

/**
 * Describes the message thinkspace.api.v1.SpaceInfo.
 * Use `create(SpaceInfoSchema)` to create a new message.
 */
export const SpaceInfoSchema: GenMessage<SpaceInfo> = /*@__PURE__*/
  messageDesc(file_src_thinkspace_api_v1_events, 0);

/**
 * @generated from message thinkspace.api.v1.LedgerEvent
 */
export type LedgerEvent = Message<"thinkspace.api.v1.LedgerEvent"> & {
  /**
   * @generated from field: string id = 1;
   */
  id: string;

  /**
   * @generated from field: string timestamp = 2;
   */
  timestamp: string;

  /**
   * @generated from field: string type = 3;
   */
  type: string;

  /**
   * @generated from field: string content = 4;
   */
  content: string;

  /**
   * @generated from field: map<string, string> metadata = 5;
   */
  metadata: { [key: string]: string };
};

/**
 * Describes the message thinkspace.api.v1.LedgerEvent.
 * Use `create(LedgerEventSchema)` to create a new message.
 */
export const LedgerEventSchema: GenMessage<LedgerEvent> = /*@__PURE__*/
  messageDesc(file_src_thinkspace_api_v1_events, 1);

/**
 * @generated from message thinkspace.api.v1.DigestMeta
 */
export type DigestMeta = Message<"thinkspace.api.v1.DigestMeta"> & {
  /**
   * @generated from field: string id = 1;
   */
  id: string;

  /**
   * @generated from field: string summary = 2;
   */
  summary: string;

  /**
   * @generated from field: bool is_sticky = 3;
   */
  isSticky: boolean;
};

/**
 * Describes the message thinkspace.api.v1.DigestMeta.
 * Use `create(DigestMetaSchema)` to create a new message.
 */
export const DigestMetaSchema: GenMessage<DigestMeta> = /*@__PURE__*/
  messageDesc(file_src_thinkspace_api_v1_events, 2);

/**
 * --- Outbound Payloads (Server -> Client) ---
 *
 * @generated from message thinkspace.api.v1.ChatStreamPayload
 */
export type ChatStreamPayload = Message<"thinkspace.api.v1.ChatStreamPayload"> & {
  /**
   * @generated from field: string text = 1;
   */
  text: string;
};

/**
 * Describes the message thinkspace.api.v1.ChatStreamPayload.
 * Use `create(ChatStreamPayloadSchema)` to create a new message.
 */
export const ChatStreamPayloadSchema: GenMessage<ChatStreamPayload> = /*@__PURE__*/
  messageDesc(file_src_thinkspace_api_v1_events, 3);

/**
 * @generated from message thinkspace.api.v1.LogMessagePayload
 */
export type LogMessagePayload = Message<"thinkspace.api.v1.LogMessagePayload"> & {
  /**
   * @generated from field: string level = 1;
   */
  level: string;

  /**
   * @generated from field: string message = 2;
   */
  message: string;
};

/**
 * Describes the message thinkspace.api.v1.LogMessagePayload.
 * Use `create(LogMessagePayloadSchema)` to create a new message.
 */
export const LogMessagePayloadSchema: GenMessage<LogMessagePayload> = /*@__PURE__*/
  messageDesc(file_src_thinkspace_api_v1_events, 4);

/**
 * @generated from message thinkspace.api.v1.AgentStreamPayload
 */
export type AgentStreamPayload = Message<"thinkspace.api.v1.AgentStreamPayload"> & {
  /**
   * @generated from field: int32 agent_id = 1;
   */
  agentId: number;

  /**
   * @generated from field: string text = 2;
   */
  text: string;
};

/**
 * Describes the message thinkspace.api.v1.AgentStreamPayload.
 * Use `create(AgentStreamPayloadSchema)` to create a new message.
 */
export const AgentStreamPayloadSchema: GenMessage<AgentStreamPayload> = /*@__PURE__*/
  messageDesc(file_src_thinkspace_api_v1_events, 5);

/**
 * @generated from message thinkspace.api.v1.SyncHistoryPayload
 */
export type SyncHistoryPayload = Message<"thinkspace.api.v1.SyncHistoryPayload"> & {
  /**
   * @generated from field: repeated thinkspace.api.v1.LedgerEvent recent_events = 1;
   */
  recentEvents: LedgerEvent[];

  /**
   * @generated from field: map<string, thinkspace.api.v1.DigestMeta> digests = 2;
   */
  digests: { [key: string]: DigestMeta };
};

/**
 * Describes the message thinkspace.api.v1.SyncHistoryPayload.
 * Use `create(SyncHistoryPayloadSchema)` to create a new message.
 */
export const SyncHistoryPayloadSchema: GenMessage<SyncHistoryPayload> = /*@__PURE__*/
  messageDesc(file_src_thinkspace_api_v1_events, 6);

/**
 * Core System Payloads
 *
 * @generated from message thinkspace.api.v1.RequestStrategyPayload
 */
export type RequestStrategyPayload = Message<"thinkspace.api.v1.RequestStrategyPayload"> & {
  /**
   * @generated from field: bool active = 1;
   */
  active: boolean;
};

/**
 * Describes the message thinkspace.api.v1.RequestStrategyPayload.
 * Use `create(RequestStrategyPayloadSchema)` to create a new message.
 */
export const RequestStrategyPayloadSchema: GenMessage<RequestStrategyPayload> = /*@__PURE__*/
  messageDesc(file_src_thinkspace_api_v1_events, 7);

/**
 * @generated from message thinkspace.api.v1.RequestReviewPayload
 */
export type RequestReviewPayload = Message<"thinkspace.api.v1.RequestReviewPayload"> & {
  /**
   * @generated from field: string branch = 1;
   */
  branch: string;
};

/**
 * Describes the message thinkspace.api.v1.RequestReviewPayload.
 * Use `create(RequestReviewPayloadSchema)` to create a new message.
 */
export const RequestReviewPayloadSchema: GenMessage<RequestReviewPayload> = /*@__PURE__*/
  messageDesc(file_src_thinkspace_api_v1_events, 8);

/**
 * @generated from message thinkspace.api.v1.AvailableSpacesPayload
 */
export type AvailableSpacesPayload = Message<"thinkspace.api.v1.AvailableSpacesPayload"> & {
  /**
   * @generated from field: repeated thinkspace.api.v1.SpaceInfo spaces = 1;
   */
  spaces: SpaceInfo[];
};

/**
 * Describes the message thinkspace.api.v1.AvailableSpacesPayload.
 * Use `create(AvailableSpacesPayloadSchema)` to create a new message.
 */
export const AvailableSpacesPayloadSchema: GenMessage<AvailableSpacesPayload> = /*@__PURE__*/
  messageDesc(file_src_thinkspace_api_v1_events, 9);

/**
 * The New Unified Flow Protocol
 *
 * @generated from message thinkspace.api.v1.FlowEventPayload
 */
export type FlowEventPayload = Message<"thinkspace.api.v1.FlowEventPayload"> & {
  /**
   * @generated from field: string flow_id = 1;
   */
  flowId: string;

  /**
   * e.g., "flow_start", "flow_spawn", "flow_status", "flow_complete"
   *
   * @generated from field: string type = 2;
   */
  type: string;

  /**
   * RFC3339 formatted time string
   *
   * @generated from field: string timestamp = 3;
   */
  timestamp: string;

  /**
   * Flow-Level Context
   *
   * @generated from field: string task_id = 4;
   */
  taskId: string;

  /**
   * @generated from field: int32 agent_count = 5;
   */
  agentCount: number;

  /**
   * Agent-Level Context
   *
   * @generated from field: string agent_id = 6;
   */
  agentId: string;

  /**
   * @generated from field: int32 agent_index = 7;
   */
  agentIndex: number;

  /**
   * @generated from field: string instruction = 8;
   */
  instruction: string;

  /**
   * e.g., "writing_code", "running_tests"
   *
   * @generated from field: string status = 9;
   */
  status: string;

  /**
   * @generated from field: int32 attempt = 10;
   */
  attempt: number;

  /**
   * Compiler output or test failures
   *
   * @generated from field: string trace = 11;
   */
  trace: string;

  /**
   * @generated from field: string candidate_id = 12;
   */
  candidateId: string;

  /**
   * @generated from field: bool passed = 13;
   */
  passed: boolean;
};

/**
 * Describes the message thinkspace.api.v1.FlowEventPayload.
 * Use `create(FlowEventPayloadSchema)` to create a new message.
 */
export const FlowEventPayloadSchema: GenMessage<FlowEventPayload> = /*@__PURE__*/
  messageDesc(file_src_thinkspace_api_v1_events, 10);

/**
 * --- Inbound Payloads (Client -> Server) ---
 *
 * @generated from message thinkspace.api.v1.SubmitPromptPayload
 */
export type SubmitPromptPayload = Message<"thinkspace.api.v1.SubmitPromptPayload"> & {
  /**
   * @generated from field: string text = 1;
   */
  text: string;

  /**
   * @generated from field: string space_id = 2;
   */
  spaceId: string;

  /**
   * Added to support dynamic chat switching
   *
   * @generated from field: string chat_id = 3;
   */
  chatId: string;
};

/**
 * Describes the message thinkspace.api.v1.SubmitPromptPayload.
 * Use `create(SubmitPromptPayloadSchema)` to create a new message.
 */
export const SubmitPromptPayloadSchema: GenMessage<SubmitPromptPayload> = /*@__PURE__*/
  messageDesc(file_src_thinkspace_api_v1_events, 11);

/**
 * @generated from message thinkspace.api.v1.SelectStrategyPayload
 */
export type SelectStrategyPayload = Message<"thinkspace.api.v1.SelectStrategyPayload"> & {
  /**
   * @generated from field: thinkspace.api.v1.DelegationStrategy strategy_id = 1;
   */
  strategyId: DelegationStrategy;
};

/**
 * Describes the message thinkspace.api.v1.SelectStrategyPayload.
 * Use `create(SelectStrategyPayloadSchema)` to create a new message.
 */
export const SelectStrategyPayloadSchema: GenMessage<SelectStrategyPayload> = /*@__PURE__*/
  messageDesc(file_src_thinkspace_api_v1_events, 12);

/**
 * @generated from message thinkspace.api.v1.ReviewDecisionPayload
 */
export type ReviewDecisionPayload = Message<"thinkspace.api.v1.ReviewDecisionPayload"> & {
  /**
   * @generated from field: string branch = 1;
   */
  branch: string;

  /**
   * @generated from field: bool accepted = 2;
   */
  accepted: boolean;
};

/**
 * Describes the message thinkspace.api.v1.ReviewDecisionPayload.
 * Use `create(ReviewDecisionPayloadSchema)` to create a new message.
 */
export const ReviewDecisionPayloadSchema: GenMessage<ReviewDecisionPayload> = /*@__PURE__*/
  messageDesc(file_src_thinkspace_api_v1_events, 13);

/**
 * --- The Master Envelope ---
 *
 * @generated from message thinkspace.api.v1.WSEvent
 */
export type WSEvent = Message<"thinkspace.api.v1.WSEvent"> & {
  /**
   * @generated from oneof thinkspace.api.v1.WSEvent.payload
   */
  payload: {
    /**
     * Outbound
     *
     * @generated from field: thinkspace.api.v1.ChatStreamPayload chat_stream = 1;
     */
    value: ChatStreamPayload;
    case: "chatStream";
  } | {
    /**
     * @generated from field: thinkspace.api.v1.LogMessagePayload log_message = 2;
     */
    value: LogMessagePayload;
    case: "logMessage";
  } | {
    /**
     * @generated from field: thinkspace.api.v1.AgentStreamPayload agent_stream = 6;
     */
    value: AgentStreamPayload;
    case: "agentStream";
  } | {
    /**
     * @generated from field: thinkspace.api.v1.RequestStrategyPayload request_strategy = 8;
     */
    value: RequestStrategyPayload;
    case: "requestStrategy";
  } | {
    /**
     * @generated from field: thinkspace.api.v1.RequestReviewPayload request_review = 9;
     */
    value: RequestReviewPayload;
    case: "requestReview";
  } | {
    /**
     * @generated from field: thinkspace.api.v1.AvailableSpacesPayload available_spaces = 10;
     */
    value: AvailableSpacesPayload;
    case: "availableSpaces";
  } | {
    /**
     * @generated from field: thinkspace.api.v1.SyncHistoryPayload sync_history = 15;
     */
    value: SyncHistoryPayload;
    case: "syncHistory";
  } | {
    /**
     * The New Unified Flow Protocol
     *
     * @generated from field: thinkspace.api.v1.FlowEventPayload flow_event = 14;
     */
    value: FlowEventPayload;
    case: "flowEvent";
  } | {
    /**
     * Inbound
     *
     * @generated from field: thinkspace.api.v1.SubmitPromptPayload submit_prompt = 11;
     */
    value: SubmitPromptPayload;
    case: "submitPrompt";
  } | {
    /**
     * @generated from field: thinkspace.api.v1.SelectStrategyPayload select_strategy = 12;
     */
    value: SelectStrategyPayload;
    case: "selectStrategy";
  } | {
    /**
     * @generated from field: thinkspace.api.v1.ReviewDecisionPayload review_decision = 13;
     */
    value: ReviewDecisionPayload;
    case: "reviewDecision";
  } | { case: undefined; value?: undefined };
};

/**
 * Describes the message thinkspace.api.v1.WSEvent.
 * Use `create(WSEventSchema)` to create a new message.
 */
export const WSEventSchema: GenMessage<WSEvent> = /*@__PURE__*/
  messageDesc(file_src_thinkspace_api_v1_events, 14);

/**
 * --- Enums ---
 *
 * @generated from enum thinkspace.api.v1.DelegationStrategy
 */
export enum DelegationStrategy {
  /**
   * @generated from enum value: DELEGATION_STRATEGY_UNSPECIFIED = 0;
   */
  UNSPECIFIED = 0,

  /**
   * @generated from enum value: DELEGATION_STRATEGY_SKIP = 1;
   */
  SKIP = 1,

  /**
   * @generated from enum value: DELEGATION_STRATEGY_MANUAL = 2;
   */
  MANUAL = 2,

  /**
   * @generated from enum value: DELEGATION_STRATEGY_REVIEW = 3;
   */
  REVIEW = 3,

  /**
   * @generated from enum value: DELEGATION_STRATEGY_REFINE = 4;
   */
  REFINE = 4,
}

/**
 * Describes the enum thinkspace.api.v1.DelegationStrategy.
 */
export const DelegationStrategySchema: GenEnum<DelegationStrategy> = /*@__PURE__*/
  enumDesc(file_src_thinkspace_api_v1_events, 0);


```


### `libs/llm/core/protos/src/thinkspace/api/v1/events.proto`
```
syntax = "proto3";

package thinkspace.api.v1;
option go_package = "github.com/tinywideclouds/thinkspace/api/v1;api_v1";

// --- Enums ---
enum DelegationStrategy {
  DELEGATION_STRATEGY_UNSPECIFIED = 0;
  DELEGATION_STRATEGY_SKIP = 1;
  DELEGATION_STRATEGY_MANUAL = 2;
  DELEGATION_STRATEGY_REVIEW = 3;
  DELEGATION_STRATEGY_REFINE = 4;
}

// --- Shared Types ---
message SpaceInfo {
  string id = 1;
  string name = 2;
  bool is_configured = 3;
}

message LedgerEvent {
  string id = 1;
  string timestamp = 2;
  string type = 3;
  string content = 4;
  map<string, string> metadata = 5;
}

message DigestMeta {
  string id = 1;
  string summary = 2;
  bool is_sticky = 3;
}

// --- Outbound Payloads (Server -> Client) ---
message ChatStreamPayload {
  string text = 1;
}

message LogMessagePayload {
  string level = 1;
  string message = 2;
}

message AgentStreamPayload {
  int32 agent_id = 1;
  string text = 2;
}

message SyncHistoryPayload {
  repeated LedgerEvent recent_events = 1;
  map<string, DigestMeta> digests = 2;
}

// Core System Payloads
message RequestStrategyPayload {
  bool active = 1;
}

message RequestReviewPayload {
  string branch = 1;
}

message AvailableSpacesPayload {
  repeated SpaceInfo spaces = 1;
}

// The New Unified Flow Protocol
message FlowEventPayload {
  string flow_id = 1;
  string type = 2;          // e.g., "flow_start", "flow_spawn", "flow_status", "flow_complete"
  string timestamp = 3;     // RFC3339 formatted time string
  
  // Flow-Level Context
  string task_id = 4;
  int32 agent_count = 5;
  
  // Agent-Level Context
  string agent_id = 6;
  int32 agent_index = 7;
  string instruction = 8;
  string status = 9;        // e.g., "writing_code", "running_tests"
  int32 attempt = 10;
  string trace = 11;        // Compiler output or test failures
  string candidate_id = 12;
  bool passed = 13;
}

// --- Inbound Payloads (Client -> Server) ---
message SubmitPromptPayload {
  string text = 1;
  string space_id = 2;
  string chat_id = 3; // Added to support dynamic chat switching
}

message SelectStrategyPayload {
  DelegationStrategy strategy_id = 1;
}

message ReviewDecisionPayload {
  string branch = 1;
  bool accepted = 2;
}

// --- The Master Envelope ---
message WSEvent {
  oneof payload {
    // Outbound
    ChatStreamPayload chat_stream = 1;
    LogMessagePayload log_message = 2;
    AgentStreamPayload agent_stream = 6;
    RequestStrategyPayload request_strategy = 8;
    RequestReviewPayload request_review = 9;
    AvailableSpacesPayload available_spaces = 10;
    SyncHistoryPayload sync_history = 15;

    // The New Unified Flow Protocol
    FlowEventPayload flow_event = 14;

    // Inbound
    SubmitPromptPayload submit_prompt = 11;
    SelectStrategyPayload select_strategy = 12;
    ReviewDecisionPayload review_decision = 13;
  }
}
```


### `libs/llm/core/transport/eslint.config.mjs`
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


### `libs/llm/core/transport/project.json`
```
{
  "name": "llm-core-transport",
  "$schema": "../../../../node_modules/nx/schemas/project-schema.json",
  "sourceRoot": "libs/llm/core/transport/src",
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


### `libs/llm/core/transport/README.md`
```
# llm-core-transport

This library provides the low-level WebSocket connection management for the ThinkSpace application.

## Responsibilities
* Manage the RxJS `WebSocketSubject` lifecycle (connect, disconnect, retry).
* Handle the raw JSON stringification and parsing of the `WSEvent` Protobuf envelope using `@bufbuild/protobuf` (via `toJsonString` and `fromJson`).
* Safely emit raw Protobuf messages to the upper layers.

## Rules
* **DO NOT** import domain logic or state management here.
* This library operates strictly on Protobuf types. It does not know what the application does with the data.
```


### `libs/llm/core/transport/tsconfig.json`
```
{
  "extends": "../../../../tsconfig.base.json",
  "compilerOptions": {
    "isolatedModules": true,
    "target": "esnext",
    "strict": true,
    "noImplicitOverride": true,
    "noPropertyAccessFromIndexSignature": true,
    "noImplicitReturns": true,
    "noFallthroughCasesInSwitch": true,
    "emitDecoratorMetadata": false,
    "module": "esnext"
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


### `libs/llm/core/transport/tsconfig.lib.json`
```
{
  "extends": "./tsconfig.json",
  "compilerOptions": {
    "outDir": "../../../../dist/out-tsc",
    "declaration": true,
    "declarationMap": true,
    "forceConsistentCasingInFileNames": true,
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


### `libs/llm/core/transport/tsconfig.spec.json`
```
{
  "extends": "./tsconfig.json",
  "compilerOptions": {
    "outDir": "../../../../dist/out-tsc",
    "forceConsistentCasingInFileNames": true,
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


### `libs/llm/core/transport/vite.config.mts`
```
/// <reference types='vitest' />
import { defineConfig } from 'vite';
import angular from '@analogjs/vite-plugin-angular';

export default defineConfig(() => ({
  root: __dirname,
  cacheDir: '../../../../node_modules/.vite/libs/llm/core/transport',
  resolve: {
    tsconfigPaths: true,
  },
  plugins: [angular()],
  test: {
    name: 'llm-core-transport',
    watch: false,
    globals: true,
    environment: 'jsdom',
    include: ['{src,tests}/**/*.{test,spec}.{js,mjs,cjs,ts,mts,cts,jsx,tsx}'],
    setupFiles: ['src/test-setup.ts'],
    reporters: ['default'],
    coverage: {
      reportsDirectory: '../../../../coverage/libs/llm/core/transport',
      provider: 'v8' as const,
    },
  },
}));
```


### `libs/llm/core/transport/src/index.ts`
```
export * from './lib/transport.service';

```


### `libs/llm/core/transport/src/test-setup.ts`
```
import '@angular/compiler';
import '@analogjs/vitest-angular/setup-snapshots';
import { setupTestBed } from '@analogjs/vitest-angular/setup-testbed';

setupTestBed({ zoneless: false });

```


### `libs/llm/core/transport/src/lib/transport.service.spec.ts`
```
import { TestBed } from '@angular/core/testing';
import { TransportService } from './transport.service';
import { create } from '@bufbuild/protobuf';
import { WSEventSchema } from '@org/llm-core-protos';
import { take } from 'rxjs/operators';
import { firstValueFrom, Subscription } from 'rxjs';
import { describe, beforeEach, afterEach, it, expect, vi, Mock } from 'vitest';

// RxJS binds directly to these properties, so we must expose them in the mock
interface MockWebSocket {
  send: Mock;
  close: Mock;
  readyState: 0 | 1 | 2 | 3;
  onmessage: ((ev: any) => any) | null;
  onopen: ((ev: any) => any) | null;
  onclose: ((ev: any) => any) | null;
  onerror: ((ev: any) => any) | null;
}

describe('TransportService', () => {
  let service: TransportService;
  let mockWebSocketInstance: MockWebSocket;
  let sub: Subscription | null = null;

  beforeEach(() => {
    mockWebSocketInstance = {
      send: vi.fn(),
      close: vi.fn(),
      readyState: 1, // WebSocket.OPEN
      onmessage: null,
      onopen: null,
      onclose: null,
      onerror: null,
    };

    class DummyWebSocket {
      constructor() {
        return mockWebSocketInstance;
      }
    }

    vi.stubGlobal('WebSocket', DummyWebSocket);

    TestBed.configureTestingModule({
      providers: [TransportService]
    });
    service = TestBed.inject(TransportService);
  });

  afterEach(() => {
    if (sub) {
      sub.unsubscribe();
      sub = null;
    }
    vi.unstubAllGlobals();
    service.disconnect();
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });

  it('should deserialize inbound JSON to a WSEvent proto', async () => {
    // 1. Subscribe to force RxJS to instantiate the socket
    const eventPromise = firstValueFrom(
      service.connect('ws://localhost:8080/ws').pipe(take(1))
    );

    // 2. Simulate the socket opening (RxJS buffers messages until open)
    if (mockWebSocketInstance.onopen) {
      mockWebSocketInstance.onopen({ type: 'open' });
    }

    // 3. Simulate an inbound payload from Go
    if (mockWebSocketInstance.onmessage) {
      mockWebSocketInstance.onmessage({
        data: JSON.stringify({
          chatStream: { text: 'Inbound test' }
        })
      });
    } else {
      throw new Error('RxJS did not attach a message listener (onmessage) to the WebSocket');
    }

    // 4. Verify translation
    const event = await eventPromise;
    expect(event.payload.case).toBe('chatStream');
    if (event.payload.case === 'chatStream') {
      expect(event.payload.value.text).toBe('Inbound test');
    }
  });

  it('should serialize outbound WSEvent proto to JSON', () => {
    // 1. Subscribe to trigger socket creation
    sub = service.connect('ws://localhost:8080/ws').subscribe();

    // 2. Mark socket as open so RxJS flushes its outbound queue
    if (mockWebSocketInstance.onopen) {
      mockWebSocketInstance.onopen({ type: 'open' });
    }

    const proto = create(WSEventSchema, {
      payload: {
        case: 'submitPrompt',
        value: { text: 'Outbound test', spaceId: 'golang' }
      }
    });

    service.send(proto);

    // 3. Verify it hit the mock's send method
    expect(mockWebSocketInstance.send).toHaveBeenCalledWith(
      JSON.stringify({
        submitPrompt: { text: 'Outbound test', spaceId: 'golang' }
      })
    );
  });

  it('should close the socket on disconnect', () => {
    // 1. Subscribe to instantiate
    sub = service.connect('ws://localhost:8080/ws').subscribe();
    
    // 2. Simulate the socket opening so RxJS registers it as fully connected
    if (mockWebSocketInstance.onopen) {
      mockWebSocketInstance.onopen({ type: 'open' });
    }

    // 3. Disconnect
    service.disconnect();
    
    // 4. Verify RxJS called close() on the mock
    expect(mockWebSocketInstance.close).toHaveBeenCalled();
  });
});
```


### `libs/llm/core/transport/src/lib/transport.service.ts`
```
import { Injectable } from '@angular/core';
import { webSocket, WebSocketSubject } from 'rxjs/webSocket';
import { Observable } from 'rxjs';
import { WSEvent, WSEventSchema } from '@org/llm-core-protos';
import { fromJson, toJsonString } from '@bufbuild/protobuf';

@Injectable({ providedIn: 'root' })
export class TransportService {
  private socket$: WebSocketSubject<WSEvent> | null = null;

  /**
   * Establishes a WebSocket connection and returns an Observable of strictly typed WSEvent protos.
   * Utilizes custom serializers to bridge RxJS with Buf's Protobuf encoding.
   */
  public connect(url: string): Observable<WSEvent> {
    if (!this.socket$ || this.socket$.closed) {
      this.socket$ = webSocket<WSEvent>({
        url,
        // Inbound: Parse raw JSON from the server and hydrate it into a WSEvent class
        deserializer: (e: MessageEvent) => {
          try {
            const parsed = JSON.parse(e.data);
            return fromJson(WSEventSchema, parsed);
          } catch (err) {
            console.error('[TransportService] 🚨 Deserialization Error!', err);
            console.error('[TransportService] 📦 Raw string from Go:', e.data);
            throw err; // Re-throw to let RxJS handle the teardown
          }
        },
        serializer: (value: WSEvent) => {
          return toJsonString(WSEventSchema, value);
        },
        openObserver: {
          next: () => console.log(`[TransportService] Connected to ${url}`)
        },
        closeObserver: {
          next: () => console.log(`[TransportService] Disconnected from ${url}`)
        }
      });
    }
    return this.socket$.asObservable();
  }

  /**
   * Pushes a WSEvent through the active WebSocket connection.
   */
  public send(message: WSEvent): void {
    if (this.socket$ && !this.socket$.closed) {
      this.socket$.next(message);
    } else {
      console.error('[TransportService] Cannot send message: WebSocket is not open.');
    }
  }

  /**
   * Completes the RxJS subject, closing the underlying WebSocket connection.
   */
  public disconnect(): void {
    if (this.socket$) {
      this.socket$.complete();
      this.socket$ = null;
    }
  }
}
```
