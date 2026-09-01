# contexter-ui

This library contains the "dumb" presentation components for the Contexter tool suite. It acts strictly as the visual layer, relying entirely on input properties for data and output events for user interactions.

## Responsibilities
* **Pure Presentation:** Renders the user interface, including the workspace file tree, selection panels, action bars, and modal overlays[cite: 1, 2].
* **Stateless Design:** Maintains no application state and makes no HTTP calls. All data is passed down via `@Input()` bindings, and all user actions are bubbled up via `@Output()` EventEmitters[cite: 1].
* **Reusability:** Components are highly decoupled and scoped purely to their visual functionality, making them easy to test and reuse across different feature layouts.

## Architecture & Boundaries
This library is tagged with `scope:contexter` and `type:ui`. Per the workspace linting rules, it is strictly isolated to the Contexter tool ecosystem.

**Allowed Dependencies:**
* `@org/contexter-shared` (for domain interfaces like `FileNode` and `ContextConfig`)[cite: 1]

**Restrictions:**
As a UI library, this module must **not** import from `@org/contexter-data-access` or `@org/contexter-feature`. It must remain entirely ignorant of how state is managed, how files are generated, or how the backend API operates.

## Provided Components
* `FileTreeComponent`: Recursive folder and file navigation[cite: 1].
* `ActionPanelComponent`: Sidebar action buttons (Generate, Save, Load, Clear)[cite: 1].
* `SelectionPanelComponent`: Visual list of active file selections with missing-file warnings[cite: 1].
* `OutputOverlayComponent`: Markdown preview modal with copy and save capabilities[cite: 1].
* `SelectionModalComponent`: Dialog for managing saved `.yaml` selection bundles[cite: 1].
* `SnackbarComponent`: Temporary toast notifications[cite: 1].

## Usage
Import individual components as needed into your smart feature components:

\`\`\`typescript
import { FileTreeComponent, ActionPanelComponent } from '@org/contexter-ui';

@Component({
  standalone: true,
  imports: [FileTreeComponent, ActionPanelComponent],
  template: `
    <lib-action-panel 
      [selectedCount]="count" 
      (clearSelection)="onClear()">
    </lib-action-panel>
  `
})
\`\`\`

## Testing
Run `nx test contexter-ui` to execute the unit tests[cite: 1].