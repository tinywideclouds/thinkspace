# contexter-ui

The Angular frontend application for the Contexter tool suite. This application serves as a visual file explorer and bundle manager, allowing developers to select repository files and compile them into LLM-ready context chunks.

## Architecture

This application acts as a thin routing shell and structural layout. It delegates all business logic, state management, and reusable UI components to dedicated libraries within the Nx workspace.

*   **State & API Integration:** Managed by `@org/contexter-data-access`.
*   **UI Components:** Consumes reusable elements (like the recursive file tree) from `@org/contexter-ui`.
*   **Module Boundaries:** Governed by the `scope:contexter` tag. It strictly avoids importing modules from `scope:llm` or `scope:shop` to prevent architectural coupling.

## Prerequisites

The UI relies on the Node API backend to access the physical file system and read the `.llm-context.yaml` configuration. Ensure the backend is running before serving the frontend:

\`\`\`bash
nx serve contexter-app
\`\`\`

## Running the Application

Start the development server for the UI:

\`\`\`bash
nx serve contexter-ui
\`\`\`

Navigate to `http://localhost:4200/`. The application will automatically reload if you change any of the source files.

## Testing

Execute the unit test suite via Jest:

\`\`\`bash
nx test contexter-ui
\`\`\`