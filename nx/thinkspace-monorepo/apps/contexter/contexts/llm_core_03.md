

### `libs\llm\core\facade\eslint.config.mjs`
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


### `libs\llm\core\facade\project.json`
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


### `libs\llm\core\facade\README.md`
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


### `libs\llm\core\facade\tsconfig.json`
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


### `libs\llm\core\facade\tsconfig.lib.json`
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


### `libs\llm\core\facade\tsconfig.spec.json`
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


### `libs\llm\core\facade\vite.config.mts`
```
/// <reference types='vitest' />
import { defineConfig } from 'vite';
import angular from '@analogjs/vite-plugin-angular';
import tsconfigPaths from 'vite-tsconfig-paths';

export default defineConfig(() => ({
  root: __dirname,
  cacheDir: '../../../../node_modules/.vite/libs/llm/core/facade',
  plugins: [angular(), tsconfigPaths()],
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


### `libs\llm\core\facade\src\index.ts`
```
export * from './lib/domain-models';
export * from './lib/facade';
```


### `libs\llm\core\facade\src\test-setup.ts`
```
import '@angular/compiler';
import '@analogjs/vitest-angular/setup-snapshots';
import { setupTestBed } from '@analogjs/vitest-angular/setup-testbed';

setupTestBed({ zoneless: false });

```


### `libs\llm\core\facade\src\lib\domain-models.ts`
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
  | { type: 'available_spaces'; spaces: { id: string; name: string }[] }
  | DomainFlowEvent;

export enum DomainDelegationStrategy {
  SKIP = 1,
  MANUAL = 2,
  REVIEW = 3,
  REFINE = 4,
}
```


### `libs\llm\core\facade\src\lib\facade.spec.ts`
```
import { describe, it, expect } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { WSEventSchema, DelegationStrategy } from '@org/llm-core-protos';
import { LlmFacade } from './facade';
import { DomainDelegationStrategy } from './domain-models';

describe('LlmFacade', () => {
  describe('toDomain (Inbound)', () => {
    it('should map a chat stream event correctly', () => {
      const proto = create(WSEventSchema, {
        payload: {
          case: 'chatStream',
          value: { text: 'Hello from model' }
        }
      });
      const domain = LlmFacade.toDomain(proto);
      expect(domain).toEqual({ type: 'chat_stream', text: 'Hello from model' });
    });

    it('should map an available spaces event correctly', () => {
      const proto = create(WSEventSchema, {
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

      const domain = LlmFacade.toDomain(proto);
      expect(domain).toEqual({
        type: 'available_spaces',
        spaces: [
          { id: 'golang', name: 'Go Developer' },
          { id: 'angular', name: 'Angular Expert' }
        ]
      });
    });

    it('should map a flow event correctly', () => {
      const proto = create(WSEventSchema, {
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

      const domain = LlmFacade.toDomain(proto);
      expect(domain).toEqual({
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
      const emptyProto = create(WSEventSchema);
      expect(LlmFacade.toDomain(emptyProto)).toBeNull();
    });
  });

  describe('Outbound Builders', () => {
    it('should create a valid SubmitPrompt WSEvent', () => {
      const proto = LlmFacade.createSubmitPrompt('Write a test', 'golang');
      
      expect(proto.payload.case).toBe('submitPrompt');
      if (proto.payload.case === 'submitPrompt') {
        expect(proto.payload.value.text).toBe('Write a test');
        expect(proto.payload.value.spaceId).toBe('golang');
      }
    });

    it('should create a valid SelectStrategy WSEvent', () => {
      const proto = LlmFacade.createSelectStrategy(DomainDelegationStrategy.REFINE);
      
      expect(proto.payload.case).toBe('selectStrategy');
      if (proto.payload.case === 'selectStrategy') {
        expect(proto.payload.value.strategyId).toBe(DelegationStrategy.REFINE);
      }
    });

    it('should create a valid ReviewDecision WSEvent', () => {
      const proto = LlmFacade.createReviewDecision('candidate/123', true);
      
      expect(proto.payload.case).toBe('reviewDecision');
      if (proto.payload.case === 'reviewDecision') {
        expect(proto.payload.value.branch).toBe('candidate/123');
        expect(proto.payload.value.accepted).toBe(true);
      }
    });
  });
});
```


### `libs\llm\core\facade\src\lib\facade.ts`
```
import { create } from '@bufbuild/protobuf';
import { 
  WSEvent, 
  WSEventSchema,
  DelegationStrategy 
} from '@org/llm-core-protos';
import { DomainEvent, DomainDelegationStrategy } from './domain-models';

export class LlmFacade {
  static toDomain(proto: WSEvent): DomainEvent | null {
    if (!proto.payload || proto.payload.case === undefined) {
      return null;
    }

    switch (proto.payload.case) {
      case 'chatStream':
        return { type: 'chat_stream', text: proto.payload.value.text };
      
      case 'logMessage':
        return { type: 'log_message', level: proto.payload.value.level, message: proto.payload.value.message };
      
      case 'delegationStart':
        return { type: 'delegation_start', agentCount: proto.payload.value.agentCount, instructions: proto.payload.value.instructions };
      
      case 'delegationComplete':
        return { type: 'delegation_complete', summary: proto.payload.value.summary };
      
      case 'agentStart':
        return { type: 'agent_start', agentId: proto.payload.value.agentId, instructions: proto.payload.value.instructions };
      
      case 'agentStream':
        return { type: 'agent_stream', agentId: proto.payload.value.agentId, text: proto.payload.value.text };
      
      case 'agentComplete':
        return { type: 'agent_complete', agentId: proto.payload.value.agentId, branch: proto.payload.value.branch, verified: proto.payload.value.verified };
      
      case 'requestStrategy':
        return { type: 'request_strategy', active: proto.payload.value.active };
      
      case 'requestReview':
        return { type: 'request_review', branch: proto.payload.value.branch };
      
      case 'availableSpaces':
        return { type: 'available_spaces', spaces: proto.payload.value.spaces.map(s => ({ id: s.id, name: s.name })) };
      
      case 'flowEvent':
        return {
          type: 'flow_event',
          flowId: proto.payload.value.flowId,
          eventType: proto.payload.value.type,
          timestamp: proto.payload.value.timestamp,
          taskId: proto.payload.value.taskId,
          agentCount: proto.payload.value.agentCount,
          agentId: proto.payload.value.agentId,
          agentIndex: proto.payload.value.agentIndex,
          instruction: proto.payload.value.instruction,
          status: proto.payload.value.status,
          attempt: proto.payload.value.attempt,
          trace: proto.payload.value.trace,
          candidateId: proto.payload.value.candidateId,
          passed: proto.payload.value.passed
        };
      
      default:
        console.warn(`[LlmFacade] Unhandled inbound proto case: ${proto.payload.case}`);
        return null;
    }
  }

  static createSubmitPrompt(text: string, spaceId: string): WSEvent {
    return create(WSEventSchema, {
      payload: {
        case: 'submitPrompt',
        value: { text, spaceId }
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


### `libs\llm\core\protos\buf.gen.yaml`
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


### `libs\llm\core\protos\eslint.config.mjs`
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


### `libs\llm\core\protos\project.json`
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


### `libs\llm\core\protos\README.md`
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


### `libs\llm\core\protos\tsconfig.json`
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


### `libs\llm\core\protos\tsconfig.lib.json`
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


### `libs\llm\core\protos\tsconfig.spec.json`
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


### `libs\llm\core\protos\vite.config.mts`
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


### `libs\llm\core\protos\src\index.ts`
```
export * from './generated/src/thinkspace/api/v1/events_pb';

```


### `libs\llm\core\protos\src\test-setup.ts`
```
import '@angular/compiler';
import '@analogjs/vitest-angular/setup-snapshots';
import { setupTestBed } from '@analogjs/vitest-angular/setup-testbed';

setupTestBed({ zoneless: false });

```


### `libs\llm\core\protos\src\generated\src\thinkspace\api\v1\events_pb.ts`
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
  fileDesc("CiJzcmMvdGhpbmtzcGFjZS9hcGkvdjEvZXZlbnRzLnByb3RvEhF0aGlua3NwYWNlLmFwaS52MSIlCglTcGFjZUluZm8SCgoCaWQYASABKAkSDAoEbmFtZRgCIAEoCSIhChFDaGF0U3RyZWFtUGF5bG9hZBIMCgR0ZXh0GAEgASgJIjMKEUxvZ01lc3NhZ2VQYXlsb2FkEg0KBWxldmVsGAEgASgJEg8KB21lc3NhZ2UYAiABKAkiQwoWRGVsZWdhdGlvblN0YXJ0UGF5bG9hZBITCgthZ2VudF9jb3VudBgBIAEoBRIUCgxpbnN0cnVjdGlvbnMYAiABKAkiLAoZRGVsZWdhdGlvbkNvbXBsZXRlUGF5bG9hZBIPCgdzdW1tYXJ5GAEgASgJIjsKEUFnZW50U3RhcnRQYXlsb2FkEhAKCGFnZW50X2lkGAEgASgFEhQKDGluc3RydWN0aW9ucxgCIAEoCSI0ChJBZ2VudFN0cmVhbVBheWxvYWQSEAoIYWdlbnRfaWQYASABKAUSDAoEdGV4dBgCIAEoCSJKChRBZ2VudENvbXBsZXRlUGF5bG9hZBIQCghhZ2VudF9pZBgBIAEoBRIOCgZicmFuY2gYAiABKAkSEAoIdmVyaWZpZWQYAyABKAgiKAoWUmVxdWVzdFN0cmF0ZWd5UGF5bG9hZBIOCgZhY3RpdmUYASABKAgiJgoUUmVxdWVzdFJldmlld1BheWxvYWQSDgoGYnJhbmNoGAEgASgJIkYKFkF2YWlsYWJsZVNwYWNlc1BheWxvYWQSLAoGc3BhY2VzGAEgAygLMhwudGhpbmtzcGFjZS5hcGkudjEuU3BhY2VJbmZvIvwBChBGbG93RXZlbnRQYXlsb2FkEg8KB2Zsb3dfaWQYASABKAkSDAoEdHlwZRgCIAEoCRIRCgl0aW1lc3RhbXAYAyABKAkSDwoHdGFza19pZBgEIAEoCRITCgthZ2VudF9jb3VudBgFIAEoBRIQCghhZ2VudF9pZBgGIAEoCRITCgthZ2VudF9pbmRleBgHIAEoBRITCgtpbnN0cnVjdGlvbhgIIAEoCRIOCgZzdGF0dXMYCSABKAkSDwoHYXR0ZW1wdBgKIAEoBRINCgV0cmFjZRgLIAEoCRIUCgxjYW5kaWRhdGVfaWQYDCABKAkSDgoGcGFzc2VkGA0gASgIIkYKE1N1Ym1pdFByb21wdFBheWxvYWQSDAoEdGV4dBgBIAEoCRIQCghzcGFjZV9pZBgCIAEoCRIPCgdjaGF0X2lkGAMgASgJIlMKFVNlbGVjdFN0cmF0ZWd5UGF5bG9hZBI6CgtzdHJhdGVneV9pZBgBIAEoDjIlLnRoaW5rc3BhY2UuYXBpLnYxLkRlbGVnYXRpb25TdHJhdGVneSI5ChVSZXZpZXdEZWNpc2lvblBheWxvYWQSDgoGYnJhbmNoGAEgASgJEhAKCGFjY2VwdGVkGAIgASgIIrgHCgdXU0V2ZW50EjsKC2NoYXRfc3RyZWFtGAEgASgLMiQudGhpbmtzcGFjZS5hcGkudjEuQ2hhdFN0cmVhbVBheWxvYWRIABI7Cgtsb2dfbWVzc2FnZRgCIAEoCzIkLnRoaW5rc3BhY2UuYXBpLnYxLkxvZ01lc3NhZ2VQYXlsb2FkSAASRQoQZGVsZWdhdGlvbl9zdGFydBgDIAEoCzIpLnRoaW5rc3BhY2UuYXBpLnYxLkRlbGVnYXRpb25TdGFydFBheWxvYWRIABJLChNkZWxlZ2F0aW9uX2NvbXBsZXRlGAQgASgLMiwudGhpbmtzcGFjZS5hcGkudjEuRGVsZWdhdGlvbkNvbXBsZXRlUGF5bG9hZEgAEjsKC2FnZW50X3N0YXJ0GAUgASgLMiQudGhpbmtzcGFjZS5hcGkudjEuQWdlbnRTdGFydFBheWxvYWRIABI9CgxhZ2VudF9zdHJlYW0YBiABKAsyJS50aGlua3NwYWNlLmFwaS52MS5BZ2VudFN0cmVhbVBheWxvYWRIABJBCg5hZ2VudF9jb21wbGV0ZRgHIAEoCzInLnRoaW5rc3BhY2UuYXBpLnYxLkFnZW50Q29tcGxldGVQYXlsb2FkSAASRQoQcmVxdWVzdF9zdHJhdGVneRgIIAEoCzIpLnRoaW5rc3BhY2UuYXBpLnYxLlJlcXVlc3RTdHJhdGVneVBheWxvYWRIABJBCg5yZXF1ZXN0X3JldmlldxgJIAEoCzInLnRoaW5rc3BhY2UuYXBpLnYxLlJlcXVlc3RSZXZpZXdQYXlsb2FkSAASRQoQYXZhaWxhYmxlX3NwYWNlcxgKIAEoCzIpLnRoaW5rc3BhY2UuYXBpLnYxLkF2YWlsYWJsZVNwYWNlc1BheWxvYWRIABI5CgpmbG93X2V2ZW50GA4gASgLMiMudGhpbmtzcGFjZS5hcGkudjEuRmxvd0V2ZW50UGF5bG9hZEgAEj8KDXN1Ym1pdF9wcm9tcHQYCyABKAsyJi50aGlua3NwYWNlLmFwaS52MS5TdWJtaXRQcm9tcHRQYXlsb2FkSAASQwoPc2VsZWN0X3N0cmF0ZWd5GAwgASgLMigudGhpbmtzcGFjZS5hcGkudjEuU2VsZWN0U3RyYXRlZ3lQYXlsb2FkSAASQwoPcmV2aWV3X2RlY2lzaW9uGA0gASgLMigudGhpbmtzcGFjZS5hcGkudjEuUmV2aWV3RGVjaXNpb25QYXlsb2FkSABCCQoHcGF5bG9hZCq3AQoSRGVsZWdhdGlvblN0cmF0ZWd5EiMKH0RFTEVHQVRJT05fU1RSQVRFR1lfVU5TUEVDSUZJRUQQABIcChhERUxFR0FUSU9OX1NUUkFURUdZX1NLSVAQARIeChpERUxFR0FUSU9OX1NUUkFURUdZX01BTlVBTBACEh4KGkRFTEVHQVRJT05fU1RSQVRFR1lfUkVWSUVXEAMSHgoaREVMRUdBVElPTl9TVFJBVEVHWV9SRUZJTkUQBEI0WjJnaXRodWIuY29tL3Rpbnl3aWRlY2xvdWRzL3RoaW5rc3BhY2UvYXBpL3YxO2FwaV92MWIGcHJvdG8z");

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
};

/**
 * Describes the message thinkspace.api.v1.SpaceInfo.
 * Use `create(SpaceInfoSchema)` to create a new message.
 */
export const SpaceInfoSchema: GenMessage<SpaceInfo> = /*@__PURE__*/
  messageDesc(file_src_thinkspace_api_v1_events, 0);

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
  messageDesc(file_src_thinkspace_api_v1_events, 1);

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
  messageDesc(file_src_thinkspace_api_v1_events, 2);

/**
 * Legacy FanOut Payloads (To be deprecated)
 *
 * @generated from message thinkspace.api.v1.DelegationStartPayload
 */
export type DelegationStartPayload = Message<"thinkspace.api.v1.DelegationStartPayload"> & {
  /**
   * @generated from field: int32 agent_count = 1;
   */
  agentCount: number;

  /**
   * @generated from field: string instructions = 2;
   */
  instructions: string;
};

/**
 * Describes the message thinkspace.api.v1.DelegationStartPayload.
 * Use `create(DelegationStartPayloadSchema)` to create a new message.
 */
export const DelegationStartPayloadSchema: GenMessage<DelegationStartPayload> = /*@__PURE__*/
  messageDesc(file_src_thinkspace_api_v1_events, 3);

/**
 * @generated from message thinkspace.api.v1.DelegationCompletePayload
 */
export type DelegationCompletePayload = Message<"thinkspace.api.v1.DelegationCompletePayload"> & {
  /**
   * @generated from field: string summary = 1;
   */
  summary: string;
};

/**
 * Describes the message thinkspace.api.v1.DelegationCompletePayload.
 * Use `create(DelegationCompletePayloadSchema)` to create a new message.
 */
export const DelegationCompletePayloadSchema: GenMessage<DelegationCompletePayload> = /*@__PURE__*/
  messageDesc(file_src_thinkspace_api_v1_events, 4);

/**
 * @generated from message thinkspace.api.v1.AgentStartPayload
 */
export type AgentStartPayload = Message<"thinkspace.api.v1.AgentStartPayload"> & {
  /**
   * @generated from field: int32 agent_id = 1;
   */
  agentId: number;

  /**
   * @generated from field: string instructions = 2;
   */
  instructions: string;
};

/**
 * Describes the message thinkspace.api.v1.AgentStartPayload.
 * Use `create(AgentStartPayloadSchema)` to create a new message.
 */
export const AgentStartPayloadSchema: GenMessage<AgentStartPayload> = /*@__PURE__*/
  messageDesc(file_src_thinkspace_api_v1_events, 5);

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
  messageDesc(file_src_thinkspace_api_v1_events, 6);

/**
 * @generated from message thinkspace.api.v1.AgentCompletePayload
 */
export type AgentCompletePayload = Message<"thinkspace.api.v1.AgentCompletePayload"> & {
  /**
   * @generated from field: int32 agent_id = 1;
   */
  agentId: number;

  /**
   * @generated from field: string branch = 2;
   */
  branch: string;

  /**
   * @generated from field: bool verified = 3;
   */
  verified: boolean;
};

/**
 * Describes the message thinkspace.api.v1.AgentCompletePayload.
 * Use `create(AgentCompletePayloadSchema)` to create a new message.
 */
export const AgentCompletePayloadSchema: GenMessage<AgentCompletePayload> = /*@__PURE__*/
  messageDesc(file_src_thinkspace_api_v1_events, 7);

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
  messageDesc(file_src_thinkspace_api_v1_events, 8);

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
  messageDesc(file_src_thinkspace_api_v1_events, 9);

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
  messageDesc(file_src_thinkspace_api_v1_events, 10);

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
  messageDesc(file_src_thinkspace_api_v1_events, 11);

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
  messageDesc(file_src_thinkspace_api_v1_events, 12);

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
  messageDesc(file_src_thinkspace_api_v1_events, 13);

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
  messageDesc(file_src_thinkspace_api_v1_events, 14);

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
     * @generated from field: thinkspace.api.v1.DelegationStartPayload delegation_start = 3;
     */
    value: DelegationStartPayload;
    case: "delegationStart";
  } | {
    /**
     * @generated from field: thinkspace.api.v1.DelegationCompletePayload delegation_complete = 4;
     */
    value: DelegationCompletePayload;
    case: "delegationComplete";
  } | {
    /**
     * @generated from field: thinkspace.api.v1.AgentStartPayload agent_start = 5;
     */
    value: AgentStartPayload;
    case: "agentStart";
  } | {
    /**
     * @generated from field: thinkspace.api.v1.AgentStreamPayload agent_stream = 6;
     */
    value: AgentStreamPayload;
    case: "agentStream";
  } | {
    /**
     * @generated from field: thinkspace.api.v1.AgentCompletePayload agent_complete = 7;
     */
    value: AgentCompletePayload;
    case: "agentComplete";
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
  messageDesc(file_src_thinkspace_api_v1_events, 15);

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


### `libs\llm\core\protos\src\thinkspace\api\v1\events.proto`
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
}

// --- Outbound Payloads (Server -> Client) ---
message ChatStreamPayload {
  string text = 1;
}

message LogMessagePayload {
  string level = 1;
  string message = 2;
}

// Legacy FanOut Payloads (To be deprecated)
message DelegationStartPayload {
  int32 agent_count = 1;
  string instructions = 2;
}

message DelegationCompletePayload {
  string summary = 1;
}

message AgentStartPayload {
  int32 agent_id = 1;
  string instructions = 2;
}

message AgentStreamPayload {
  int32 agent_id = 1;
  string text = 2;
}

message AgentCompletePayload {
  int32 agent_id = 1;
  string branch = 2;
  bool verified = 3;
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
    DelegationStartPayload delegation_start = 3;
    DelegationCompletePayload delegation_complete = 4;
    AgentStartPayload agent_start = 5;
    AgentStreamPayload agent_stream = 6;
    AgentCompletePayload agent_complete = 7;
    RequestStrategyPayload request_strategy = 8;
    RequestReviewPayload request_review = 9;
    AvailableSpacesPayload available_spaces = 10;

    // The New Unified Flow Protocol
    FlowEventPayload flow_event = 14;

    // Inbound
    SubmitPromptPayload submit_prompt = 11;
    SelectStrategyPayload select_strategy = 12;
    ReviewDecisionPayload review_decision = 13;
  }
}
```


### `libs\llm\core\transport\eslint.config.mjs`
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


### `libs\llm\core\transport\project.json`
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


### `libs\llm\core\transport\README.md`
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


### `libs\llm\core\transport\tsconfig.json`
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


### `libs\llm\core\transport\tsconfig.lib.json`
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


### `libs\llm\core\transport\tsconfig.spec.json`
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


### `libs\llm\core\transport\vite.config.mts`
```
/// <reference types='vitest' />
import { defineConfig } from 'vite';
import angular from '@analogjs/vite-plugin-angular';
import tsconfigPaths from 'vite-tsconfig-paths';

export default defineConfig(() => ({
  root: __dirname,
  cacheDir: '../../../../node_modules/.vite/libs/llm/core/transport',
  plugins: [angular(), tsconfigPaths()],
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


### `libs\llm\core\transport\src\index.ts`
```
export * from './lib/transport.service';

```


### `libs\llm\core\transport\src\test-setup.ts`
```
import '@angular/compiler';
import '@analogjs/vitest-angular/setup-snapshots';
import { setupTestBed } from '@analogjs/vitest-angular/setup-testbed';

setupTestBed({ zoneless: false });

```


### `libs\llm\core\transport\src\lib\transport.service.spec.ts`
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


### `libs\llm\core\transport\src\lib\transport.service.ts`
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
