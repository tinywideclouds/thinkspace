# contexter-shared

This library contains the core domain interfaces and shared models for the Contexter tool suite. It acts as the strict typing contract between the `contexter-app` (Node API backend) and the Angular UI frontend.

## Responsibilities
* Define data structures for recursive file navigation (`FileNode`).
* Define configuration schemas for `.llm-context.yaml` persistence (`ContextConfig`, `BundlePreset`).
* Define API request and response payloads (`BundleRequest`, `BundleResponse`).

## Architecture & Boundaries
This library is tagged with `scope:contexter` and `scope:shared`[cite: 30]. Per the workspace linting rules defined in `eslint.config.mjs`, it is strictly isolated to the Contexter tool ecosystem and can only depend on other `scope:contexter` libraries[cite: 28, 30]. It must not import from or be imported by the `scope:llm` or `scope:shop` domains[cite: 28].

## Usage
Import models via the workspace alias defined in `tsconfig.base.json`[cite: 29]:

\`\`\`typescript
import { FileNode, ContextConfig } from '@org/contexter-shared';
\`\`\`

## Testing
Run `nx test contexter-shared` to execute the unit tests[cite: 31].