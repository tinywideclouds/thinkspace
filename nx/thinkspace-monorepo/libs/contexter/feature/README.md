# contexter-feature

This library contains the "smart" feature components for the Contexter tool suite. It acts as the orchestration layer, bridging the reactive domain state with the visual presentation layer.

## Responsibilities
* **State Orchestration:** Injects and coordinates domain-specific services (`WorkspaceService`, `SelectionService`, `BundleService`) from `@org/contexter-data-access`.
* **Component Composition:** Assembles the structural application layout using the "dumb" presentation components provided by `@org/contexter-ui`.
* **Application Entry:** Serves as the primary routed feature or shell entry point for the `contexter-ng` application.

## Architecture & Boundaries
This library is tagged with `scope:contexter` and `type:feature`. Per the workspace linting rules defined in `eslint.config.mjs`, it is strictly isolated to the Contexter tool ecosystem. 

**Allowed Dependencies:**
* `@org/contexter-data-access` (for state and API interactions)
* `@org/contexter-ui` (for presentation components)
* `@org/contexter-shared` (for interfaces and models)

**Restrictions:**
As a feature library, this module must **not** be imported by any other library (like `data-access` or `ui`) to prevent circular dependencies. It should only be consumed by the end application (`contexter-ng`).

## Usage
Import the main layout component into the application shell:

\`\`\`typescript
import { Component } from '@angular/core';
import { LayoutComponent } from '@org/contexter-feature';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [LayoutComponent],
  template: '<lib-layout></lib-layout>'
})
export class App {}
\`\`\`

## Testing
Run `nx test contexter-feature` to execute the unit tests.