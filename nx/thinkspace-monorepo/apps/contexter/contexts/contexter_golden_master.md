

### `libs\contexter\ui\src\lib\action-panel\action-panel.component.html`
```
<div class="p-4 bg-white border-b border-gray-200">
  <div class="flex flex-col gap-3">
    
    <button 
      (click)="generateContext.emit()"
      [disabled]="selectedCount === 0"
      class="w-full flex items-center justify-center gap-2 bg-blue-600 hover:bg-blue-700 disabled:bg-gray-300 text-white font-bold py-2 px-4 rounded transition-colors shadow-sm">
      <span>⚡</span> Generate LLM Context
    </button>
    
    <hr class="border-gray-100 my-1" />

    <div class="flex flex-col gap-1.5">
      <span class="text-[10px] font-bold text-gray-400 uppercase tracking-wider">Selection Bundle</span>
      <div class="flex gap-2">
        <button 
          (click)="saveSelection.emit()"
          [disabled]="selectedCount === 0"
          class="flex-1 bg-gray-100 hover:bg-gray-200 disabled:bg-gray-50 disabled:text-gray-400 text-gray-700 font-medium py-1.5 px-2 rounded transition-colors text-sm">
          Save
        </button>
        
        <button 
          (click)="loadSelection.emit('temp_placeholder')"
          class="flex-1 bg-gray-100 hover:bg-gray-200 text-gray-700 font-medium py-1.5 px-2 rounded transition-colors text-sm">
          Load
        </button>
        
        <button 
          (click)="clearSelection.emit()"
          [disabled]="selectedCount === 0"
          class="flex-1 bg-red-50 hover:bg-red-100 disabled:bg-gray-50 disabled:text-gray-400 text-red-600 font-medium py-1.5 px-2 rounded transition-colors text-sm">
          Clear
        </button>
      </div>
    </div>

    <div class="text-xs text-gray-500 text-center mt-1">
      {{ selectedCount }} files selected
    </div>
  </div>
</div>
```


### `libs\contexter\ui\src\lib\action-panel\action-panel.component.ts`
```
import { Component, EventEmitter, Input, Output } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'lib-action-panel',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './action-panel.component.html'
})
export class ActionPanelComponent {
  @Input() config: any;
  @Input() selectedCount = 0;

  @Output() generateContext = new EventEmitter<void>();
  @Output() saveSelection = new EventEmitter<void>();
  @Output() loadSelection = new EventEmitter<string>();
  @Output() clearSelection = new EventEmitter<void>();
}
```


### `libs\contexter\ui\src\lib\file-tree\file-tree.component.html`
```
<ul class="list-none pl-4 border-l border-gray-200">
  @for (node of nodes; track node.path) {
    <li class="my-1">
      @if (node.isDirectory) {
        <div 
          class="cursor-pointer font-medium text-gray-700 flex items-center gap-2 p-1.5 rounded hover:bg-blue-50 hover:text-blue-700 transition-colors select-none"
          (click)="toggleExpand(node)">
          <span class="text-lg leading-none">{{ expandedNodes.has(node.path) ? '📂' : '📁' }}</span>
          
          @if (hasDirectFiles(node)) {
            <input 
              type="checkbox" 
              [checked]="isAllDirectFilesSelected(node)"
              [indeterminate]="isSomeDirectFilesSelected(node)"
              (click)="$event.preventDefault(); $event.stopPropagation(); directoryToggled.emit(node)"
              class="form-checkbox h-4 w-4 text-blue-600 rounded cursor-pointer ml-1"
              title="Select all files in this directory"
            />
          }
          
          <span class="truncate">{{ node.name }}</span>
          
          @if (expandedNodes.has(node.path) && !node.children) {
            <span class="text-xs text-blue-500 font-semibold animate-pulse ml-2">Loading...</span>
          }
        </div>
        
        @if (expandedNodes.has(node.path) && node.children) {
          <lib-file-tree 
            [nodes]="node.children" 
            [selectedFiles]="selectedFiles"
            [expandedNodes]="expandedNodes"
            (fileToggled)="fileToggled.emit($event)"
            (folderToggled)="folderToggled.emit($event)"
            (directoryToggled)="directoryToggled.emit($event)"
            (expandToggled)="expandToggled.emit($event)">
          </lib-file-tree>
        }
      } @else {
        <label class="flex items-center gap-2 cursor-pointer text-gray-600 p-1.5 rounded hover:bg-gray-100 transition-colors select-none">
          <input 
            type="checkbox" 
            [checked]="selectedFiles.has(node.path)"
            (change)="fileToggled.emit(node.path)"
            class="form-checkbox h-4 w-4 text-blue-600 rounded cursor-pointer"
          />
          <span class="text-sm">📄 {{ node.name }}</span>
        </label>
      }
    </li>
  }
</ul>
```


### `libs\contexter\ui\src\lib\file-tree\file-tree.component.ts`
```
import { Component, EventEmitter, Input, Output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FileNode } from '@org/contexter-shared';

@Component({
  selector: 'lib-file-tree',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './file-tree.component.html',
})
export class FileTreeComponent {
  @Input() nodes: FileNode[] = [];
  @Input() selectedFiles: Set<string> = new Set();
  @Input() expandedNodes: Set<string> = new Set();
  
  @Output() fileToggled = new EventEmitter<string>();
  @Output() folderToggled = new EventEmitter<FileNode>();
  @Output() directoryToggled = new EventEmitter<FileNode>();
  @Output() expandToggled = new EventEmitter<FileNode>();

  toggleExpand(node: FileNode) {
    this.expandToggled.emit(node);
    if (!node.children) {
      this.folderToggled.emit(node);
    }
  }

  hasDirectFiles(node: FileNode): boolean {
    return !!node.children && node.children.some(c => !c.isDirectory);
  }

  isAllDirectFilesSelected(node: FileNode): boolean {
    if (!node.children) return false;
    const files = node.children.filter(c => !c.isDirectory);
    if (files.length === 0) return false;
    return files.every(f => this.selectedFiles.has(f.path));
  }

  isSomeDirectFilesSelected(node: FileNode): boolean {
    if (!node.children) return false;
    const files = node.children.filter(c => !c.isDirectory);
    if (files.length === 0) return false;
    
    const selectedCount = files.filter(f => this.selectedFiles.has(f.path)).length;
    return selectedCount > 0 && selectedCount < files.length;
  }
}
```


### `libs\contexter\ui\src\lib\output-overlay\output-overlay.component.ts`
```
import { Component, EventEmitter, Output, input, signal, computed } from '@angular/core';
import { CommonModule } from '@angular/common';

export interface SaveContextPayload {
  filename: string;
  description: string;
}

@Component({
  selector: 'lib-output-overlay',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './output-overlay.component.html'
})
export class OutputOverlayComponent {
  content = input<string>('');
  defaultFilename = input<string>('context_001.md');
  suggestedNewFilename = input<string>('');
  existingFiles = input<string[]>([]);

  @Output() copy = new EventEmitter<string>();
  @Output() save = new EventEmitter<SaveContextPayload>();
  @Output() close = new EventEmitter<void>();

  isSaveOpen = signal(false);
  customFilename = '';
  customDescription = '';
  private userHasEditedFilename = false;

  estimatedTokens = computed(() => Math.ceil(this.content().length / 4));
  
  estimatedFileSize = computed(() => {
    const bytes = this.content().length;
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  });

  incrementedFilename = computed(() => {
    const def = this.defaultFilename();
    if (!def) return '';
    const match = def.match(/^(.*?)_?(\d*)(\.[a-zA-Z0-9]+)$/);
    if (!match) return def;
    
    const base = match[1];
    const numStr = match[2];
    const ext = match[3];
    
    if (!numStr) return `${base}_01${ext}`;
    
    const num = parseInt(numStr, 10) + 1;
    const paddedNum = num.toString().padStart(numStr.length, '0');
    return `${base}_${paddedNum}${ext}`;
  });

  get canSaveAsNew(): boolean {
    return this.existingFiles().includes(this.defaultFilename()) && this.customFilename !== this.incrementedFilename();
  }

  toggleSavePanel() {
    this.isSaveOpen.update(v => !v);
    if (this.isSaveOpen()) {
      this.customFilename = this.defaultFilename();
      this.userHasEditedFilename = false;
      this.customDescription = '';
    }
  }

  updateFilename(event: Event) {
    this.customFilename = (event.target as HTMLInputElement).value;
    this.userHasEditedFilename = true;
  }

  updateDescription(event: Event) {
    this.customDescription = (event.target as HTMLInputElement).value;
  }

  onFilenameFocus() {
    if (!this.userHasEditedFilename && !this.existingFiles().includes(this.defaultFilename())) {
      this.customFilename = '';
    }
    this.userHasEditedFilename = true;
  }

  useIncrementedName() {
    this.customFilename = this.incrementedFilename();
    this.userHasEditedFilename = true;
  }

  willOverwrite(): boolean {
    const name = (this.userHasEditedFilename && this.customFilename.trim()) 
      ? this.customFilename.trim() 
      : this.defaultFilename();
    let finalName = name;
    if (finalName && !finalName.endsWith('.md')) finalName += '.md';
    return this.existingFiles().includes(finalName);
  }

  submitSave() {
    let filename = (this.userHasEditedFilename && this.customFilename.trim()) 
      ? this.customFilename.trim() 
      : this.defaultFilename();
      
    if (filename && !filename.endsWith('.md')) filename += '.md';
      
    this.save.emit({
      filename,
      description: this.customDescription.trim()
    });
    this.isSaveOpen.set(false);
  }

  onClose() {
    this.isSaveOpen.set(false);
    this.close.emit();
  }
}
```


### `libs\contexter\ui\src\lib\selection-panel\selection-panel.component.html`
```
<div class="flex flex-col h-full bg-gray-50">
  <div class="p-4 border-b border-gray-200 bg-gray-100 flex justify-between items-center">
    <div class="flex items-center gap-3">
      <h3 class="text-xs font-bold text-gray-500 uppercase tracking-wider">Selected Files</h3>
      @if (currentFilename) {
        <div class="flex items-center gap-1">
          <span 
            class="text-[10px] font-bold text-blue-700 bg-blue-100 px-2 py-0.5 rounded border border-blue-200"
            title="Currently loaded selection">
            {{ currentFilename }}
          </span>
          @if (hasUnsavedChanges) {
            <button 
              (click)="quickSave.emit()"
              class="text-[10px] font-bold text-orange-700 bg-orange-100 hover:bg-orange-200 px-1.5 py-0.5 rounded border border-orange-200 transition-colors"
              title="Overwrite current file with changes">
              [modified]
            </button>
          }
        </div>
      }
    </div>
    
    @if (missingFiles.size > 0) {
      <div class="flex items-center gap-2">
        <span class="text-red-500 font-bold cursor-help" title="Some files in your selection are no longer available on disk.">[!]</span>
        <button 
          (click)="removeMissing.emit()"
          class="text-[10px] uppercase tracking-wide font-bold bg-red-100 hover:bg-red-200 text-red-700 px-2 py-1 rounded transition-colors"
          title="Remove missing files from selection">
          Clean
        </button>
      </div>
    }
  </div>
  
  <ul class="flex-1 overflow-y-auto p-4 space-y-2 text-sm">
    @for (file of files; track file) {
      <li class="flex items-start gap-2 group">
        <button 
          (click)="removeFile.emit(file)" 
          class="text-red-400 hover:text-red-600 font-bold px-1 rounded hover:bg-red-50 transition-colors shrink-0">
          ×
        </button>
        <code 
          class="bg-white border px-1.5 py-0.5 rounded text-gray-700 break-all transition-colors"
          [class.border-red-300]="missingFiles.has(file)"
          [class.text-red-500]="missingFiles.has(file)"
          [class.line-through]="missingFiles.has(file)"
          [class.border-gray-200]="!missingFiles.has(file)">
          {{ file }}
          @if (missingFiles.has(file)) {
            <span class="text-red-500 ml-1 inline-block" title="File not found on disk">⚠️</span>
          }
        </code>
      </li>
    } @empty {
      <li class="text-gray-400 italic text-center mt-10">No files selected.</li>
    }
  </ul>
</div>
```


### `libs\contexter\ui\src\lib\selection-panel\selection-panel.component.ts`
```
import { Component, EventEmitter, Input, Output } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'lib-selection-panel',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './selection-panel.component.html',
})
export class SelectionPanelComponent {
  @Input() files: Set<string> = new Set();
  @Input() missingFiles: Set<string> = new Set();
  @Input() currentFilename: string | null = null;
  @Input() hasUnsavedChanges = false;
  
  @Output() removeFile = new EventEmitter<string>();
  @Output() removeMissing = new EventEmitter<void>();
  @Output() quickSave = new EventEmitter<void>();
}
```


### `libs\contexter\ui\src\lib\snackbar\snackbar.component.ts`
```
import { Component, Input } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'lib-snackbar',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div 
      class="fixed bottom-6 left-1/2 transform -translate-x-1/2 transition-all duration-300 z-[100] flex items-center gap-2 px-4 py-3 bg-gray-900 text-white text-sm font-medium rounded-lg shadow-xl"
      [class.opacity-0]="!isVisible"
      [class.translate-y-12]="!isVisible"
      [class.opacity-100]="isVisible"
      [class.translate-y-0]="isVisible"
      [style.pointer-events]="isVisible ? 'auto' : 'none'">
      @if (type === 'success') {
        <span class="text-green-400">✓</span>
      } @else {
        <span class="text-red-400 font-bold">⚠️</span>
      }
      {{ message }}
    </div>
  `
})
export class SnackbarComponent {
  @Input() message = '';
  @Input() isVisible = false;
  @Input() type: 'success' | 'error' = 'success';
}
```


### `libs\contexter\shared\src\lib\models.ts`
```
export interface FileNode {
  name: string;
  path: string;
  isDirectory: boolean;
  children?: FileNode[];
}

export interface BundlePreset {
  description?: string;
  files: string[];
  last_context?: string;
}

export interface ContextConfig {
  root_dir: string;
  always_load: string[];
  saved_bundles: Record<string, BundlePreset>;
  paths: {
    selections: string;
    contexts: string;
  };
  counters: {
    selections: number;
    contexts: number;
  };
}

export interface BundleRequest {
  files: string[];
}

export interface BundleResponse {
  content: string;
  count: number;
}
```


### `apps\contexter\contexter-node\src\main.ts`
```
import express from 'express';
import cors from 'cors';
import { apiRouter } from './routes/api.routes';

const app = express();
app.use(cors());

// Increase the payload limit for large context bundles
app.use(express.json({ limit: '50mb' }));
app.use(express.urlencoded({ limit: '50mb', extended: true }));

// Mount the router
app.use('/api', apiRouter);

const port = process.env['PORT'] || 3333;
const server = app.listen(port, () => {
  console.log(`Listening at http://localhost:${port}/api`);
});
server.on('error', console.error);
```


### `apps\contexter\contexter-node\src\routes\api.routes.ts`
```
import { Router } from 'express';
import * as path from 'path';
import * as fs from 'fs/promises';
import * as yaml from 'js-yaml';
import { ensureDir, getNextSequenceFilename, getExistingFiles } from '../services/file-system';
import { getConfig, saveConfig } from '../services/config';
import { validateFiles, generateBundleContent } from '../services/workspace';

export const apiRouter = Router();

apiRouter.get('/config', async (req, res) => {
  try {
    res.json(await getConfig());
  } catch (error) {
    res.status(500).json({ error: 'Failed to load config' });
  }
});

apiRouter.post('/config', async (req, res) => {
  try {
    await saveConfig(req.body);
    res.json({ success: true });
  } catch (error) {
    res.status(500).json({ error: 'Failed to save config' });
  }
});

apiRouter.get('/tree', async (req, res) => {
  try {
    const dir = req.query['dir'] as string || './';
    const entries = await fs.readdir(dir, { withFileTypes: true });
    const nodes = entries.map(entry => ({
      name: entry.name,
      path: path.join(dir, entry.name),
      isDirectory: entry.isDirectory()
    }));
    res.json(nodes);
  } catch (error) {
    res.status(500).json({ error: 'Failed to read directory' });
  }
});

apiRouter.get('/defaults', async (req, res) => {
  try {
    const config = await getConfig();
    const nextContext = await getNextSequenceFilename(config.paths.contexts, 'context_', '.md');
    const nextSelection = await getNextSequenceFilename(config.paths.selections, 'selection_', '.yaml');
    const existingContexts = await getExistingFiles(config.paths.contexts);
    const existingSelections = await getExistingFiles(config.paths.selections);
    res.json({ nextContext, nextSelection, existingContexts, existingSelections });
  } catch (error) {
    res.status(500).json({ error: 'Failed to calculate default filenames' });
  }
});

apiRouter.post('/validate', async (req, res) => {
  try {
    const missing = await validateFiles(req.body.files);
    res.json({ missing });
  } catch (error) {
    res.status(500).json({ error: 'Failed to validate files' });
  }
});

apiRouter.post('/generate', async (req, res) => {
  try {
    const content = await generateBundleContent(req.body.files);
    res.json({ content, count: req.body.files.length });
  } catch (error) {
    res.status(500).json({ error: 'Failed to generate payload' });
  }
});

apiRouter.get('/selections', async (req, res) => {
  try {
    const config = await getConfig();
    const files = await getExistingFiles(config.paths.selections);
    res.json(files.filter(f => f.endsWith('.yaml')));
  } catch (error) {
    res.status(500).json({ error: 'Failed to list selections' });
  }
});

apiRouter.get('/selections/:filename', async (req, res) => {
  try {
    const config = await getConfig();
    const filePath = path.join(config.paths.selections, req.params.filename);
    const content = await fs.readFile(filePath, 'utf8');
    const parsed = yaml.load(content) as any;
    res.json(parsed);
  } catch (error) {
    res.status(500).json({ error: 'Failed to load selection' });
  }
});

apiRouter.post('/selections', async (req, res) => {
  try {
    const { files, filename: customFilename, description, last_context } = req.body;
    const config = await getConfig();
    const targetDir = await ensureDir(config.paths.selections);
    
    let filename = customFilename?.trim() || await getNextSequenceFilename(targetDir, 'selection_', '.yaml');
    if (!filename.endsWith('.yaml')) filename += '.yaml';
    
    const data: any = { files };
    if (description) data.description = description;
    if (last_context) data.last_context = last_context;

    await fs.writeFile(path.join(targetDir, filename), yaml.dump(data), 'utf8');
    res.json({ success: true, filename });
  } catch (error) {
    res.status(500).json({ error: 'Failed to save selection' });
  }
});

apiRouter.post('/contexts', async (req, res) => {
  try {
    const { markdown, filename: customFilename, description } = req.body;
    const config = await getConfig();
    const targetDir = await ensureDir(config.paths.contexts);
    
    let filename = customFilename?.trim() || await getNextSequenceFilename(targetDir, 'context_', '.md');
    if (!filename.endsWith('.md')) filename += '.md';
    
    const finalContent = description ? `<!-- Description: ${description} -->\n\n${markdown}` : markdown;
    await fs.writeFile(path.join(targetDir, filename), finalContent, 'utf8');
    res.json({ success: true, filename });
  } catch (error) {
    res.status(500).json({ error: 'Failed to save context' });
  }
});
```


### `apps\contexter\contexter-node\src\services\config.ts`
```
import * as fs from 'fs/promises';
import * as path from 'path';
import * as yaml from 'js-yaml';
import { ContextConfig } from '@org/contexter-shared';

const CONFIG_PATH = path.join(process.cwd(), 'default.yaml');

const DEFAULT_CONFIG: ContextConfig = {
  root_dir: './',
  always_load: [],
  saved_bundles: {},
  paths: {
    selections: 'apps/contexter/bundles',
    contexts: 'apps/contexter/contexts'
  },
  counters: {
    selections: 1,
    contexts: 1
  }
};

export async function ensureConfigExists() {
  try {
    await fs.access(CONFIG_PATH);
  } catch {
    await fs.mkdir(path.dirname(CONFIG_PATH), { recursive: true });
    await fs.writeFile(CONFIG_PATH, yaml.dump(DEFAULT_CONFIG), 'utf8');
  }
}

export async function getConfig(): Promise<ContextConfig> {
  await ensureConfigExists();
  const file = await fs.readFile(CONFIG_PATH, 'utf8');
  return yaml.load(file) as ContextConfig;
}

export async function saveConfig(config: ContextConfig): Promise<void> {
  await ensureConfigExists();
  await fs.writeFile(CONFIG_PATH, yaml.dump(config), 'utf8');
}
```


### `apps\contexter\contexter-node\src\services\file-system.ts`
```
import * as fs from 'fs/promises';

export async function ensureDir(dirPath: string): Promise<string> {
  await fs.mkdir(dirPath, { recursive: true });
  return dirPath;
}

// Add this to your existing file-system.ts exports
export async function getExistingFiles(dirPath: string): Promise<string[]> {
  try {
    await ensureDir(dirPath);
    return await fs.readdir(dirPath);
  } catch {
    return [];
  }
}
/**
 * Scans a directory for files matching prefix_XXX.ext and returns the next logical filename.
 * Example: getNextSequenceFilename('./contexts', 'context_', '.md') -> 'context_003.md'
 */
export async function getNextSequenceFilename(
  dirPath: string, 
  prefix: string, 
  extension: string
): Promise<string> {
  await ensureDir(dirPath);
  
  try {
    const files = await fs.readdir(dirPath);
    let maxCount = 0;
    
    // Regex to match exact pattern: e.g., ^context_(\d+)\.md$
    const regex = new RegExp(`^${prefix}(\\d+)${extension.replace('.', '\\.')}$`);
    
    for (const file of files) {
      const match = file.match(regex);
      if (match) {
        const num = parseInt(match[1], 10);
        if (num > maxCount) {
          maxCount = num;
        }
      }
    }
    
    const nextCount = (maxCount + 1).toString().padStart(3, '0');
    return `${prefix}${nextCount}${extension}`;
  } catch (error) {
    console.error(`[FS] Error reading directory ${dirPath}:`, error);
    // Safe fallback if directory is unreadable
    return `${prefix}001${extension}`;
  }
}
```


### `apps\contexter\contexter-node\src\services\workspace.ts`
```
import * as fs from 'fs/promises';

export async function validateFiles(files: string[]): Promise<string[]> {
  const missing = [];
  for (const file of files) {
    try {
      await fs.stat(file);
    } catch {
      missing.push(file);
    }
  }
  return missing;
}

export async function generateBundleContent(files: string[]): Promise<string> {
  let content = '';
  for (const file of files) {
    try {
      const fileContent = await fs.readFile(file, 'utf8');
      content += `\n\n### \`${file}\`\n\`\`\`\n${fileContent}\n\`\`\`\n`;
    } catch (e) {
      content += `\n\n### \`${file}\`\n<!-- FILE NOT FOUND: ${file} -->\n`;
    }
  }
  return content;
}
```


### `apps\contexter\contexter-ng\src\app\app.config.ts`
```
import {
  ApplicationConfig,
  provideBrowserGlobalErrorListeners,
} from '@angular/core';
import { provideRouter } from '@angular/router';
import { appRoutes } from './app.routes';

export const appConfig: ApplicationConfig = {
  providers: [provideBrowserGlobalErrorListeners(), provideRouter(appRoutes)],
};

```


### `apps\contexter\contexter-ng\src\app\app.css`
```

```


### `apps\contexter\contexter-ng\src\app\app.html`
```
<lib-layout></lib-layout>
```


### `apps\contexter\contexter-ng\src\app\app.routes.ts`
```
import { Route } from '@angular/router';

export const appRoutes: Route[] = [];

```


### `apps\contexter\contexter-ng\src\app\app.spec.ts`
```
import { TestBed } from '@angular/core/testing';
import { App } from './app';
import { NxWelcome } from './nx-welcome';

describe('App', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [App, NxWelcome],
    }).compileComponents();
  });

  it('should render title', async () => {
    const fixture = TestBed.createComponent(App);
    await fixture.whenStable();
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.querySelector('h1')?.textContent).toContain(
      'Welcome contexter-ng',
    );
  });
});

```


### `apps\contexter\contexter-ng\src\app\app.ts`
```
import { Component } from '@angular/core';
import { LayoutComponent } from '@org/contexter-feature'; // adjust import path to match your Nx workspace setup

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [LayoutComponent],
  templateUrl: './app.html'
})
export class App {}
```


### `apps\contexter\contexter-node\project.json`
```
{
  "name": "contexter-app",
  "$schema": "../../../node_modules/nx/schemas/project-schema.json",
  "sourceRoot": "apps/contexter/contexter-node/src",
  "projectType": "application",
  "tags": ["scope:contexter"],
  "targets": {
    "build": {
      "executor": "@nx/esbuild:esbuild",
      "outputs": ["{options.outputPath}"],
      "defaultConfiguration": "production",
      "options": {
        "platform": "node",
        "outputPath": "dist/apps/contexter/contexter-node",
        "format": ["cjs"],
        "bundle": false,
        "main": "apps/contexter/contexter-node/src/main.ts",
        "tsConfig": "apps/contexter/contexter-node/tsconfig.app.json",
        "assets": ["apps/contexter/contexter-node/src/assets"],
        "generatePackageJson": true,
        "esbuildOptions": {
          "sourcemap": true,
          "outExtension": {
            ".js": ".js"
          }
        }
      },
      "configurations": {
        "development": {},
        "production": {
          "esbuildOptions": {
            "sourcemap": false,
            "outExtension": {
              ".js": ".js"
            }
          }
        }
      }
    },
    "prune-lockfile": {
      "dependsOn": ["build"],
      "cache": true,
      "executor": "@nx/js:prune-lockfile",
      "outputs": [
        "{workspaceRoot}/dist/apps/contexter/contexter-node/package.json",
        "{workspaceRoot}/dist/apps/contexter/contexter-node/package-lock.json"
      ],
      "options": {
        "buildTarget": "build"
      }
    },
    "copy-workspace-modules": {
      "dependsOn": ["build"],
      "cache": true,
      "outputs": [
        "{workspaceRoot}/dist/apps/contexter/contexter-node/workspace_modules"
      ],
      "executor": "@nx/js:copy-workspace-modules",
      "options": {
        "buildTarget": "build"
      }
    },
    "prune": {
      "dependsOn": ["prune-lockfile", "copy-workspace-modules"],
      "executor": "nx:noop"
    },
    "serve": {
      "continuous": true,
      "executor": "@nx/js:node",
      "defaultConfiguration": "development",
      "dependsOn": ["build"],
      "options": {
        "buildTarget": "contexter-app:build",
        "runBuildTargetDependencies": false
      },
      "configurations": {
        "development": {
          "buildTarget": "contexter-app:build:development"
        },
        "production": {
          "buildTarget": "contexter-app:build:production"
        }
      }
    }
  }
}

```


### `apps\contexter\contexter-node\eslint.config.mjs`
```
import baseConfig from '../../../eslint.config.mjs';

export default [...baseConfig];

```


### `apps\contexter\contexter-node\tsconfig.app.json`
```
{
  "extends": "./tsconfig.json",
  "compilerOptions": {
    "outDir": "../../../dist/out-tsc",
    "module": "commonjs",
    "types": ["node"]
  },
  "include": ["src/**/*.ts"]
}

```


### `apps\contexter\contexter-node\tsconfig.json`
```
{
  "extends": "../../../tsconfig.base.json",
  "files": [],
  "include": [],
  "references": [
    {
      "path": "./tsconfig.app.json"
    }
  ],
  "compilerOptions": {
    "esModuleInterop": true
  }
}

```


### `libs\contexter\data-access\src\index.ts`
```
export * from './lib/bundle.service';
export * from './lib/selection.service';
export * from './lib/workspace.service';


```


### `libs\contexter\data-access\src\test-setup.ts`
```
import '@angular/compiler';
import '@analogjs/vitest-angular/setup-snapshots';
import { setupTestBed } from '@analogjs/vitest-angular/setup-testbed';

setupTestBed();

```


### `libs\contexter\data-access\eslint.config.mjs`
```
import nx from '@nx/eslint-plugin';
import baseConfig from '../../../eslint.config.mjs';

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


### `libs\contexter\data-access\project.json`
```
{
  "name": "contexter-data-access",
  "$schema": "../../../node_modules/nx/schemas/project-schema.json",
  "sourceRoot": "libs/contexter/data-access/src",
  "prefix": "lib",
  "projectType": "library",
  "tags": ["scope:contexter"],
  "targets": {
    "lint": {
      "executor": "@nx/eslint:lint"
    }
  }
}

```


### `libs\contexter\data-access\README.md`
```
# contexter-data-access

This library was generated with [Nx](https://nx.dev).

## Running unit tests

Run `nx test contexter-data-access` to execute the unit tests.

```


### `libs\contexter\data-access\tsconfig.json`
```
{
  "extends": "../../../tsconfig.base.json",
  "compilerOptions": {
    "isolatedModules": true,
    "target": "esnext",
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


### `libs\contexter\data-access\tsconfig.lib.json`
```
{
  "extends": "./tsconfig.json",
  "compilerOptions": {
    "outDir": "../../../dist/out-tsc",
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


### `libs\contexter\data-access\tsconfig.spec.json`
```
{
  "extends": "./tsconfig.json",
  "compilerOptions": {
    "outDir": "../../../dist/out-tsc",
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


### `libs\contexter\data-access\vite.config.mts`
```
/// <reference types='vitest' />
import { defineConfig } from 'vite';
import angular from '@analogjs/vite-plugin-angular';

export default defineConfig(() => ({
  root: __dirname,
  cacheDir: '../../../node_modules/.vite/libs/contexter/data-access',
  resolve: { tsconfigPaths: true },
  plugins: [angular()],
  test: {
    name: 'contexter-data-access',
    watch: false,
    globals: true,
    environment: 'jsdom',
    include: ['{src,tests}/**/*.{test,spec}.{js,mjs,cjs,ts,mts,cts,jsx,tsx}'],
    setupFiles: ['src/test-setup.ts'],
    reporters: ['default'],
    coverage: {
      reportsDirectory: '../../../coverage/libs/contexter/data-access',
      provider: 'v8' as const,
    },
  },
}));
```


### `libs\contexter\shared\src\index.ts`
```
export * from './lib/models';

```


### `libs\contexter\shared\src\test-setup.ts`
```
import '@angular/compiler';
import '@analogjs/vitest-angular/setup-snapshots';
import { setupTestBed } from '@analogjs/vitest-angular/setup-testbed';

setupTestBed({ zoneless: false });

```


### `libs\contexter\shared\eslint.config.mjs`
```
import nx from '@nx/eslint-plugin';
import baseConfig from '../../../eslint.config.mjs';

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


### `libs\contexter\shared\project.json`
```
{
  "name": "contexter-shared",
  "$schema": "../../../node_modules/nx/schemas/project-schema.json",
  "sourceRoot": "libs/contexter/shared/src",
  "prefix": "lib",
  "projectType": "library",
  "tags": ["scope:contexter", "scope:shared"],
  "targets": {
    "lint": {
      "executor": "@nx/eslint:lint"
    }
  }
}

```


### `libs\contexter\shared\README.md`
```
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
```


### `libs\contexter\shared\tsconfig.json`
```
{
  "extends": "../../../tsconfig.base.json",
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


### `libs\contexter\shared\tsconfig.lib.json`
```
{
  "extends": "./tsconfig.json",
  "compilerOptions": {
    "outDir": "../../../dist/out-tsc",
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


### `libs\contexter\shared\tsconfig.spec.json`
```
{
  "extends": "./tsconfig.json",
  "compilerOptions": {
    "outDir": "../../../dist/out-tsc",
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


### `libs\contexter\shared\vite.config.mts`
```
/// <reference types='vitest' />
import { defineConfig } from 'vite';
import angular from '@analogjs/vite-plugin-angular';
import { nxViteTsPaths } from '@nx/vite/plugins/nx-tsconfig-paths.plugin';
import { nxCopyAssetsPlugin } from '@nx/vite/plugins/nx-copy-assets.plugin';

export default defineConfig(() => ({
  root: __dirname,
  cacheDir: '../../../node_modules/.vite/libs/contexter/shared',
  plugins: [angular(), nxViteTsPaths(), nxCopyAssetsPlugin(['*.md'])],
  // Uncomment this if you are using workers.
  // worker: {
  //   plugins: () => [ nxViteTsPaths() ],
  // },
  test: {
    name: 'contexter-shared',
    watch: false,
    globals: true,
    environment: 'jsdom',
    include: ['{src,tests}/**/*.{test,spec}.{js,mjs,cjs,ts,mts,cts,jsx,tsx}'],
    setupFiles: ['src/test-setup.ts'],
    reporters: ['default'],
    coverage: {
      reportsDirectory: '../../../coverage/libs/contexter/shared',
      provider: 'v8' as const,
    },
  },
}));

```


### `libs\contexter\ui\src\lib\output-overlay\output-overlay.component.html`
```
@if (content()) {
  <div class="absolute inset-0 z-50 bg-black/50 flex items-center justify-center p-8 backdrop-blur-sm">
    <div class="bg-white rounded-lg shadow-2xl w-full max-w-5xl h-full max-h-[85vh] flex flex-col overflow-hidden">
      
      <!-- Header -->
      <div class="p-4 border-b border-gray-200 flex justify-between items-center bg-gray-50 shrink-0">
        <div class="flex items-center gap-3">
          <h3 class="font-bold text-lg flex items-center gap-2">
            <span>⚡</span> LLM Context Payload
          </h3>
          <span class="text-xs font-semibold text-gray-500 bg-gray-200 px-2 py-0.5 rounded-full border border-gray-300">
            ~{{ estimatedTokens() | number }} tokens &bull; {{ estimatedFileSize() }}
          </span>
        </div>
        <div class="flex gap-3 items-center">
          <button 
            (click)="copy.emit(content())" 
            class="px-3 py-1.5 text-sm font-medium text-blue-600 hover:bg-blue-50 rounded transition-colors">
            Copy to Clipboard
          </button>
          <button 
            (click)="toggleSavePanel()" 
            class="px-3 py-1.5 text-sm font-medium bg-blue-600 text-white hover:bg-blue-700 rounded transition-colors">
            {{ isSaveOpen() ? 'Cancel Save' : 'Save to Disk' }}
          </button>
          <button 
            (click)="onClose()" 
            class="text-sm font-bold text-gray-400 hover:text-gray-700 p-1">
            ✕
          </button>
        </div>
      </div>

      <!-- Optional Save Panel -->
      @if (isSaveOpen()) {
        <div class="p-4 bg-blue-50 border-b border-blue-200 shrink-0 flex flex-col gap-3">
          <div class="text-xs font-semibold text-blue-800 uppercase tracking-wider">
            Save Context File
          </div>
          
          <div class="flex gap-3">
            <div class="flex-1">
              <label class="block text-xs text-gray-600 mb-1">Filename</label>
              <input 
                type="text" 
                [value]="customFilename"
                (input)="updateFilename($event)"
                (focus)="onFilenameFocus()"
                placeholder="e.g. context_001.md"
                class="w-full text-sm px-3 py-1.5 bg-white border border-gray-300 rounded focus:outline-none focus:border-blue-500 font-mono"
              />
              <div class="flex items-start justify-between mt-1 min-h-[20px]">
                @if (willOverwrite()) {
                  <div class="text-xs text-orange-600 font-bold flex items-center gap-1 animate-pulse">
                    <span>⚠️</span> Overwriting existing file.
                  </div>
                } @else {
                  <div></div>
                }
                
                @if (canSaveAsNew) {
                  <button 
                    (click)="useIncrementedName()" 
                    class="text-[10px] text-blue-600 hover:text-blue-800 hover:underline font-bold transition-colors text-right mt-0.5">
                    Increment File Name ({{ incrementedFilename() }})
                  </button>
                }
              </div>
            </div>
            
            <div class="flex-[2]">
              <label class="block text-xs text-gray-600 mb-1">Description (Optional)</label>
              <input 
                type="text" 
                [value]="customDescription"
                (input)="updateDescription($event)"
                placeholder="Brief description of this context payload..."
                class="w-full text-sm px-3 py-1.5 bg-white border border-gray-300 rounded focus:outline-none focus:border-blue-500"
              />
            </div>

            <div class="flex items-start">
              <button 
                (click)="submitSave()"
                class="px-4 py-1.5 bg-green-600 hover:bg-green-700 text-white text-sm font-bold rounded transition-colors mt-5">
                Confirm Save
              </button>
            </div>
          </div>
        </div>
      }

      <!-- Preview Body -->
      <div class="flex-1 overflow-auto p-4 bg-gray-900">
        <pre class="text-gray-100 text-sm whitespace-pre-wrap font-mono">{{ content() }}</pre>
      </div>

    </div>
  </div>
}
```


### `libs\contexter\ui\src\index.ts`
```
export * from './lib/file-tree/file-tree.component';
export * from './lib/action-panel/action-panel.component';
export * from './lib/selection-panel/selection-panel.component';
export * from './lib/output-overlay/output-overlay.component';
export * from './lib/snackbar/snackbar.component';
export * from './lib/selection-modal/selection-modal.component';
```


### `libs\contexter\ui\src\test-setup.ts`
```
import '@angular/compiler';
import '@analogjs/vitest-angular/setup-snapshots';
import { setupTestBed } from '@analogjs/vitest-angular/setup-testbed';

setupTestBed();

```


### `libs\contexter\ui\eslint.config.mjs`
```
import nx from '@nx/eslint-plugin';
import baseConfig from '../../../eslint.config.mjs';

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


### `libs\contexter\ui\project.json`
```
{
  "name": "contexter-ui",
  "$schema": "../../../node_modules/nx/schemas/project-schema.json",
  "sourceRoot": "libs/contexter/ui/src",
  "prefix": "lib",
  "projectType": "library",
  "tags": ["scope:contexter"],
  "targets": {
    "lint": {
      "executor": "@nx/eslint:lint"
    }
  }
}

```


### `libs\contexter\ui\README.md`
```
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
```


### `libs\contexter\ui\tsconfig.json`
```
{
  "extends": "../../../tsconfig.base.json",
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


### `libs\contexter\ui\tsconfig.lib.json`
```
{
  "extends": "./tsconfig.json",
  "compilerOptions": {
    "outDir": "../../../dist/out-tsc",
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


### `libs\contexter\ui\tsconfig.spec.json`
```
{
  "extends": "./tsconfig.json",
  "compilerOptions": {
    "outDir": "../../../dist/out-tsc",
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


### `libs\contexter\ui\vite.config.mts`
```
/// <reference types='vitest' />
import { defineConfig } from 'vite';
import angular from '@analogjs/vite-plugin-angular';

export default defineConfig(() => ({
  root: __dirname,
  cacheDir: '../../../node_modules/.vite/libs/contexter/ui',
  resolve: { tsconfigPaths: true },
  plugins: [angular()],
  test: {
    name: 'contexter-ui',
    watch: false,
    globals: true,
    environment: 'jsdom',
    include: ['{src,tests}/**/*.{test,spec}.{js,mjs,cjs,ts,mts,cts,jsx,tsx}'],
    setupFiles: ['src/test-setup.ts'],
    reporters: ['default'],
    coverage: {
      reportsDirectory: '../../../coverage/libs/contexter/ui',
      provider: 'v8' as const,
    },
  },
}));
```


### `apps\contexter\contexter-node\src\services\file-system.spec.ts`
```
import { describe, it, expect, vi, beforeEach } from 'vitest';
import * as fs from 'fs/promises';
import { getNextSequenceFilename, getExistingFiles, ensureDir } from './file-system';

vi.mock('fs/promises');

describe('File System Service', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    // Supress expected console.errors in the test output
    vi.spyOn(console, 'error').mockImplementation(() => {});
  });

  describe('getNextSequenceFilename', () => {
    it('should return 001 if the directory is empty', async () => {
      vi.mocked(fs.mkdir).mockResolvedValue(undefined);
      vi.mocked(fs.readdir).mockResolvedValue([] as any);

      const filename = await getNextSequenceFilename('./test', 'context_', '.md');
      
      expect(filename).toBe('context_001.md');
    });

    it('should calculate the next number based on existing sequence files', async () => {
      vi.mocked(fs.mkdir).mockResolvedValue(undefined);
      vi.mocked(fs.readdir).mockResolvedValue([
        'context_001.md',
        'context_005.md',
        'random_file.txt'
      ] as any);

      const filename = await getNextSequenceFilename('./test', 'context_', '.md');
      
      // Should find 5 as max, return 6 padded to 006
      expect(filename).toBe('context_006.md');
    });

    it('should fallback to 001 if the directory read fails', async () => {
      vi.mocked(fs.mkdir).mockResolvedValue(undefined);
      vi.mocked(fs.readdir).mockRejectedValue(new Error('EACCES: permission denied'));

      const filename = await getNextSequenceFilename('./forbidden', 'selection_', '.yaml');
      
      expect(filename).toBe('selection_001.yaml');
      expect(console.error).toHaveBeenCalled();
    });
  });

  describe('getExistingFiles', () => {
    it('should return an array of file names on success', async () => {
      vi.mocked(fs.mkdir).mockResolvedValue(undefined);
      vi.mocked(fs.readdir).mockResolvedValue(['fileA.txt', 'fileB.txt'] as any);

      const files = await getExistingFiles('./test');
      
      expect(files).toEqual(['fileA.txt', 'fileB.txt']);
    });

    it('should catch errors and return an empty array', async () => {
      vi.mocked(fs.readdir).mockRejectedValue(new Error('Cannot read'));

      const files = await getExistingFiles('./test');
      
      expect(files).toEqual([]);
    });
  });
});
```


### `apps\contexter\contexter-node\src\services\workspace.spec.ts`
```
import { describe, it, expect, vi, beforeEach } from 'vitest';
import * as fs from 'fs/promises';
import { validateFiles, generateBundleContent } from './workspace';

vi.mock('fs/promises');

describe('Workspace Service', () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  describe('validateFiles', () => {
    it('should return an empty array when all files exist', async () => {
      vi.mocked(fs.stat).mockResolvedValue({} as any);

      const missing = await validateFiles(['file1.ts', 'file2.ts']);
      
      expect(missing).toEqual([]);
      expect(fs.stat).toHaveBeenCalledTimes(2);
    });

    it('should return an array of missing file paths when stat fails', async () => {
      vi.mocked(fs.stat).mockImplementation(async (filePath) => {
        if (filePath === 'missing.ts') throw new Error('ENOENT');
        return {} as any;
      });

      const missing = await validateFiles(['exists.ts', 'missing.ts']);
      
      expect(missing).toEqual(['missing.ts']);
    });
  });

  describe('generateBundleContent', () => {
    it('should wrap successfully read file content in markdown blocks', async () => {
      vi.mocked(fs.readFile).mockResolvedValue('console.log("hello");');

      const content = await generateBundleContent(['script.js']);
      
      expect(content).toContain('### `script.js`');
      expect(content).toContain('```\nconsole.log("hello");\n```');
    });

    it('should inject a FILE NOT FOUND comment when a file fails to read', async () => {
      vi.mocked(fs.readFile).mockRejectedValue(new Error('ENOENT'));

      const content = await generateBundleContent(['dead-file.js']);
      
      expect(content).toContain('### `dead-file.js`');
      expect(content).toContain('<!-- FILE NOT FOUND: dead-file.js -->');
    });

    it('should handle a mix of successful and failed files', async () => {
      vi.mocked(fs.readFile).mockImplementation(async (filePath) => {
        if (filePath === 'good.js') return 'const a = 1;';
        throw new Error('ENOENT');
      });

      const content = await generateBundleContent(['good.js', 'bad.js']);
      
      expect(content).toContain('```\nconst a = 1;\n```');
      expect(content).toContain('<!-- FILE NOT FOUND: bad.js -->');
    });
  });
});
```


### `libs\contexter\data-access\src\lib\bundle.service.spec.ts`
```
import { TestBed } from '@angular/core/testing';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { BundleService } from './bundle.service';
import { SelectionService } from './selection.service';
import { WorkspaceService } from './workspace.service';

describe('BundleService', () => {
  let service: BundleService;
  let selectionService: SelectionService;
  let httpMock: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [
        BundleService,
        SelectionService,
        WorkspaceService,
        provideHttpClient(),
        provideHttpClientTesting()
      ]
    });
    service = TestBed.inject(BundleService);
    selectionService = TestBed.inject(SelectionService);
    httpMock = TestBed.inject(HttpTestingController);
  });

  afterEach(() => {
    httpMock.verify();
  });

  describe('generateBundle', () => {
    it('should return empty string and not make HTTP call if no files are selected', async () => {
      selectionService.selectedFiles.set(new Set());
      
      const content = await service.generateBundle();
      
      expect(content).toBe('');
      httpMock.expectNone('http://localhost:3333/api/generate');
    });

    it('should post selected files and set the generated bundle signal', async () => {
      selectionService.selectedFiles.set(new Set(['./main.ts']));
      
      const promise = service.generateBundle();
      
      const req = httpMock.expectOne('http://localhost:3333/api/generate');
      expect(req.request.body).toEqual({ files: ['./main.ts'] });
      
      req.flush({ content: 'mock content', count: 1 });
      const result = await promise;
      
      expect(result).toBe('mock content');
      expect(service.generatedBundle()).toBe('mock content');
    });
  });
});
```


### `libs\contexter\data-access\src\lib\bundle.service.ts`
```
import { inject, Injectable, signal } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { firstValueFrom } from 'rxjs';
import { SelectionService } from './selection.service';
import { BundleResponse } from '@org/contexter-shared';

@Injectable({ providedIn: 'root' })
export class BundleService {
  private readonly http = inject(HttpClient);
  private readonly selection = inject(SelectionService);
  private readonly baseUrl = 'http://localhost:3333/api';

  readonly generatedBundle = signal<string>('');
  
  readonly currentContextFilename = signal<string | null>(null);
  readonly nextContextFilename = signal<string>('context_001.md');
  readonly nextSelectionFilename = signal<string>('selection_001.yaml');
  readonly existingContexts = signal<string[]>([]);
  readonly existingSelections = signal<string[]>([]);

  async loadDefaults() {
    try {
      const defaults = await firstValueFrom(
        this.http.get<{
          nextContext: string, nextSelection: string,
          existingContexts: string[], existingSelections: string[]
        }>(`${this.baseUrl}/defaults`)
      );
      this.nextContextFilename.set(defaults.nextContext);
      this.nextSelectionFilename.set(defaults.nextSelection);
      this.existingContexts.set(defaults.existingContexts);
      this.existingSelections.set(defaults.existingSelections);
    } catch (e) {
      console.error('[UI] Failed to load defaults:', e);
    }
  }

  async generateBundle() {
    const files = Array.from(this.selection.selectedFiles());
    if (files.length === 0) return '';
    const response = await firstValueFrom(
      this.http.post<BundleResponse>(`${this.baseUrl}/generate`, { files })
    );
    this.generatedBundle.set(response.content);
    return response.content;
  }

  async saveContextToDisk(markdown: string, filename?: string, description?: string) {
    if (!markdown) return null;
    const res = await firstValueFrom(
      this.http.post<{success: boolean, filename: string}>(`${this.baseUrl}/contexts`, { 
        markdown, filename, description
      })
    );
    if (res?.success) {
      this.currentContextFilename.set(res.filename);
    }
    return res;
  }
}
```


### `libs\contexter\data-access\src\lib\selection.service.spec.ts`
```
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { SelectionService } from './selection.service';
import { WorkspaceService } from './workspace.service';
import { FileNode } from '@org/contexter-shared';

describe('SelectionService', () => {
  let service: SelectionService;
  let httpMock: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [
        SelectionService,
        WorkspaceService,
        provideHttpClient(),
        provideHttpClientTesting()
      ]
    });
    service = TestBed.inject(SelectionService);
    httpMock = TestBed.inject(HttpTestingController);
  });

  afterEach(() => {
    httpMock.verify();
  });

  describe('validateSelection', () => {
    it('should update missingFiles signal based on backend validation', async () => {
      service.selectedFiles.set(new Set(['good.ts', 'bad.ts']));
      
      const promise = service.validateSelection();
      
      const req = httpMock.expectOne('http://localhost:3333/api/validate');
      expect(req.request.method).toBe('POST');
      expect(req.request.body).toEqual({ files: ['good.ts', 'bad.ts'] });
      
      req.flush({ missing: ['bad.ts'] });
      await promise;
      
      expect(service.missingFiles().has('bad.ts')).toBe(true);
      expect(service.missingFiles().has('good.ts')).toBe(false);
    });
  });

  describe('toggleDirectoryFiles', () => {
    it('should select all files in a directory if none or some are selected', () => {
      const node: FileNode = {
        name: 'src', path: './src', isDirectory: true,
        children: [
          { name: 'a.ts', path: './src/a.ts', isDirectory: false },
          { name: 'b.ts', path: './src/b.ts', isDirectory: false }
        ]
      };

      service.selectedFiles.set(new Set(['./src/a.ts']));
      
      service.toggleDirectoryFiles(node);
      
      httpMock.expectOne('http://localhost:3333/api/validate').flush({ missing: [] });

      expect(service.selectedFiles().has('./src/a.ts')).toBe(true);
      expect(service.selectedFiles().has('./src/b.ts')).toBe(true);
    });

    it('should deselect all files in a directory if all are currently selected', () => {
      const node: FileNode = {
        name: 'src', path: './src', isDirectory: true,
        children: [
          { name: 'a.ts', path: './src/a.ts', isDirectory: false },
          { name: 'b.ts', path: './src/b.ts', isDirectory: false }
        ]
      };

      service.selectedFiles.set(new Set(['./src/a.ts', './src/b.ts']));
      
      service.toggleDirectoryFiles(node);
      
      // Removed httpMock.expectOne() because validateSelection returns early when the list is empty

      expect(service.selectedFiles().has('./src/a.ts')).toBe(false);
      expect(service.selectedFiles().has('./src/b.ts')).toBe(false);
    });
  });

  describe('removeMissingFiles', () => {
    it('should remove all missing files from the selected files set', () => {
      service.selectedFiles.set(new Set(['good.ts', 'bad.ts']));
      service.missingFiles.set(new Set(['bad.ts']));
      
      service.removeMissingFiles();
      
      expect(service.selectedFiles().has('good.ts')).toBe(true);
      expect(service.selectedFiles().has('bad.ts')).toBe(false);
      expect(service.missingFiles().size).toBe(0);
    });
  });
});
```


### `libs\contexter\data-access\src\lib\selection.service.ts`
```
import { inject, Injectable, signal } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { firstValueFrom } from 'rxjs';
import { WorkspaceService } from './workspace.service';
import { FileNode } from '@org/contexter-shared';

@Injectable({ providedIn: 'root' })
export class SelectionService {
  private readonly http = inject(HttpClient);
  private readonly workspace = inject(WorkspaceService);
  private readonly baseUrl = 'http://localhost:3333/api';

  readonly selectedFiles = signal<Set<string>>(new Set());
  readonly missingFiles = signal<Set<string>>(new Set());
  
  readonly currentSelectionFilename = signal<string | null>(null);
  readonly selectionDescription = signal<string | undefined>(undefined);
  readonly lastContextFilename = signal<string | undefined>(undefined);
  readonly hasUnsavedChanges = signal<boolean>(false);

  private markChanged() {
    if (this.currentSelectionFilename()) {
      this.hasUnsavedChanges.set(true);
    }
  }

  updateLastContextFilename(filename: string) {
    if (this.lastContextFilename() !== filename) {
      this.lastContextFilename.set(filename);
      this.markChanged();
    }
  }

  async validateSelection() {
    const files = Array.from(this.selectedFiles());
    if (files.length === 0) {
      this.missingFiles.set(new Set());
      return;
    }
    try {
      const res = await firstValueFrom(
        this.http.post<{missing: string[]}>(`${this.baseUrl}/validate`, { files })
      );
      this.missingFiles.set(new Set(res.missing));
    } catch (e) {
      console.error('[UI] Failed to validate selection:', e);
    }
  }

  toggleFileSelection(filePath: string) {
    const current = new Set(this.selectedFiles());
    if (current.has(filePath)) current.delete(filePath);
    else current.add(filePath);
    this.selectedFiles.set(current);
    this.validateSelection();
    this.markChanged();
  }

  toggleDirectoryFiles(node: FileNode) {
    if (!node.children) return;
    const files = node.children.filter(c => !c.isDirectory);
    if (files.length === 0) return;

    const current = new Set(this.selectedFiles());
    const allSelected = files.every(f => current.has(f.path));

    if (allSelected) files.forEach(f => current.delete(f.path));
    else files.forEach(f => current.add(f.path));

    this.selectedFiles.set(current);
    this.validateSelection();
    this.markChanged();
  }

  clearSelection() {
    this.selectedFiles.set(new Set());
    this.missingFiles.set(new Set());
    this.currentSelectionFilename.set(null);
    this.selectionDescription.set(undefined);
    this.lastContextFilename.set(undefined);
    this.hasUnsavedChanges.set(false);
  }

  removeMissingFiles() {
    const current = new Set(this.selectedFiles());
    const missing = this.missingFiles();
    
    if (missing.size === 0) return;

    missing.forEach(file => current.delete(file));
    this.selectedFiles.set(current);
    this.missingFiles.set(new Set());
    this.markChanged();
  }

  async saveSelection(filename?: string, description?: string, last_context?: string) {
    const files = Array.from(this.selectedFiles());
    if (files.length === 0) return null;
    const res = await firstValueFrom(
      this.http.post<{success: boolean, filename: string}>(`${this.baseUrl}/selections`, { 
        files, filename, description, last_context 
      })
    );
    
    if (res?.success) {
      this.currentSelectionFilename.set(res.filename);
      if (description !== undefined) this.selectionDescription.set(description);
      if (last_context !== undefined) this.lastContextFilename.set(last_context);
      this.hasUnsavedChanges.set(false);
    }
    return res;
  }

  async loadSelection(filename: string) {
    try {
      const payload = await firstValueFrom(
        this.http.get<{files: string[], description?: string, last_context?: string}>(`${this.baseUrl}/selections/${filename}`)
      );
      
      const files = payload.files || [];
      this.selectedFiles.set(new Set(files));
      this.validateSelection();
      
      const dirsToLoad = new Set<string>();
      for (const file of files) {
        for (let i = 0; i < file.length; i++) {
          if (file[i] === '/' || file[i] === '\\') {
            const dir = file.substring(0, i);
            if (dir && dir !== '.') dirsToLoad.add(dir);
          }
        }
      }

      const sortedDirs = Array.from(dirsToLoad).sort((a, b) => a.length - b.length);
      const currentExpanded = new Set(this.workspace.expandedNodes());

      for (const dir of sortedDirs) {
        currentExpanded.add(dir);
        const node = this.workspace.findNode(this.workspace.fileTree(), dir);
        if (node && !node.children) {
          try {
            const children = await firstValueFrom(
              this.http.get<FileNode[]>(`${this.baseUrl}/tree?dir=${encodeURIComponent(dir)}`)
            );
            const sortedChildren = [...children].sort((a, b) => {
              if (a.isDirectory === b.isDirectory) return a.name.localeCompare(b.name);
              return a.isDirectory ? -1 : 1;
            });
            this.workspace.fileTree.update(tree => this.workspace.updateTree(tree, dir, sortedChildren));
          } catch (e) {
            console.error(`[UI] Failed to load path segment: ${dir}`);
          }
        }
      }
      
      this.workspace.expandedNodes.set(currentExpanded);
      
      this.currentSelectionFilename.set(filename);
      this.selectionDescription.set(payload.description);
      this.lastContextFilename.set(payload.last_context);
      this.hasUnsavedChanges.set(false);
      
      return true;
    } catch (e) {
      console.error('[UI] Failed to load selection:', e);
      return false;
    }
  }
}
```


### `libs\contexter\data-access\src\lib\workspace.service.spec.ts`
```
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { WorkspaceService } from './workspace.service';
import { FileNode } from '@org/contexter-shared';

describe('WorkspaceService', () => {
  let service: WorkspaceService;
  let httpMock: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [
        WorkspaceService,
        provideHttpClient(),
        provideHttpClientTesting()
      ]
    });
    service = TestBed.inject(WorkspaceService);
    httpMock = TestBed.inject(HttpTestingController);
  });

  afterEach(() => {
    httpMock.verify();
  });

  describe('Tree Identity & Updates', () => {
    it('should preserve object identity if no changes are made during update', () => {
      const initialTree: FileNode[] = [
        { name: 'src', path: './src', isDirectory: true, children: [] }
      ];
      
      const newTree = service.updateTree(initialTree, './unknown', []);
      
      expect(newTree).toBe(initialTree);
    });

    it('should update children and return a new array reference when a match is found', () => {
      const initialTree: FileNode[] = [
        { name: 'src', path: './src', isDirectory: true }
      ];
      const newChildren: FileNode[] = [
        { name: 'main.ts', path: './src/main.ts', isDirectory: false }
      ];
      
      const newTree = service.updateTree(initialTree, './src', newChildren);
      
      expect(newTree).not.toBe(initialTree);
      expect(newTree[0].children).toBe(newChildren);
    });
  });

  describe('refreshWorkspace', () => {
    it('should prune expanded nodes if they no longer exist on disk', async () => {
      service.expandedNodes.set(new Set(['./src', './deleted-folder']));
      
      const refreshPromise = service.refreshWorkspace();
      
      const reqRoot = httpMock.expectOne('http://localhost:3333/api/tree?dir=.%2F');
      reqRoot.flush([
        { name: 'src', path: './src', isDirectory: true }
      ]);

      // Yield to the event loop so the async `for` loop can trigger the next request
      await Promise.resolve();

      const reqSrc = httpMock.expectOne('http://localhost:3333/api/tree?dir=.%2Fsrc');
      reqSrc.flush([]);

      // Yield again for the final loop iteration
      await Promise.resolve();

      const reqDeleted = httpMock.expectOne('http://localhost:3333/api/tree?dir=.%2Fdeleted-folder');
      reqDeleted.flush('Not Found', { status: 404, statusText: 'Not Found' });

      await refreshPromise;

      expect(service.expandedNodes().has('./src')).toBe(true);
      expect(service.expandedNodes().has('./deleted-folder')).toBe(false);
    });
  });
});
```


### `libs\contexter\data-access\src\lib\workspace.service.ts`
```
import { inject, Injectable, signal } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { ContextConfig, FileNode } from '@org/contexter-shared';
import { firstValueFrom } from 'rxjs';

@Injectable({ providedIn: 'root' })
export class WorkspaceService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = 'http://localhost:3333/api';

  readonly config = signal<ContextConfig | null>(null);
  readonly fileTree = signal<FileNode[]>([]);
  readonly expandedNodes = signal<Set<string>>(new Set());

  private sortNodes(nodes: FileNode[]): FileNode[] {
    return [...nodes].sort((a, b) => {
      if (a.isDirectory === b.isDirectory) {
        return a.name.localeCompare(b.name);
      }
      return a.isDirectory ? -1 : 1;
    });
  }

  async loadConfig() {
    const conf = await firstValueFrom(this.http.get<ContextConfig>(`${this.baseUrl}/config`));
    this.config.set(conf);
    return conf;
  }

  async loadDirectory(dirPath?: string) {
    const targetDir = dirPath || this.config()?.root_dir || './';
    const tree = await firstValueFrom(
      this.http.get<FileNode[]>(`${this.baseUrl}/tree?dir=${encodeURIComponent(targetDir)}`)
    );
    this.fileTree.set(this.sortNodes(tree));
    this.expandedNodes.set(new Set());
  }

  async refreshWorkspace() {
    const targetDir = this.config()?.root_dir || './';
    try {
      let tree = await firstValueFrom(
        this.http.get<FileNode[]>(`${this.baseUrl}/tree?dir=${encodeURIComponent(targetDir)}`)
      );
      tree = this.sortNodes(tree);
      
      const expanded = Array.from(this.expandedNodes()).sort((a, b) => a.length - b.length);
      const validExpanded = new Set<string>();

      for (const dir of expanded) {
        try {
          const children = await firstValueFrom(
            this.http.get<FileNode[]>(`${this.baseUrl}/tree?dir=${encodeURIComponent(dir)}`)
          );
          tree = this.updateTree(tree, dir, this.sortNodes(children));
          validExpanded.add(dir);
        } catch (e) {
          console.warn(`[UI] Directory missing on disk during refresh: ${dir}`);
        }
      }

      this.fileTree.set(tree);
      this.expandedNodes.set(validExpanded);
    } catch (e) {
      console.error('[UI] Failed to refresh workspace:', e);
    }
  }

  async loadChildren(node: FileNode) {
    if (node.children) return; 
    try {
      const children = await firstValueFrom(
        this.http.get<FileNode[]>(`${this.baseUrl}/tree?dir=${encodeURIComponent(node.path)}`)
      );
      this.fileTree.update(tree => this.updateTree(tree, node.path, this.sortNodes(children)));
    } catch (error) {
      console.error(`[UI] Failed to load directory contents for ${node.path}:`, error);
    }
  }

  findNode(nodes: FileNode[], targetPath: string): FileNode | undefined {
    for (const node of nodes) {
      if (node.path === targetPath) return node;
      if (node.children) {
        const found = this.findNode(node.children, targetPath);
        if (found) return found;
      }
    }
    return undefined;
  }

  updateTree(nodes: FileNode[], targetPath: string, newChildren: FileNode[]): FileNode[] {
    let hasChanges = false;
    const newNodes = nodes.map(n => {
      if (n.path === targetPath) {
        hasChanges = true;
        return { ...n, children: newChildren };
      }
      if (n.children) {
        const updatedChildren = this.updateTree(n.children, targetPath, newChildren);
        if (updatedChildren !== n.children) {
          hasChanges = true;
          return { ...n, children: updatedChildren };
        }
      }
      return n;
    });
    return hasChanges ? newNodes : nodes;
  }

  toggleNodeExpansion(path: string) {
    const current = new Set(this.expandedNodes());
    if (current.has(path)) {
      current.delete(path);
      for (const p of current) {
        if (p.startsWith(path + '/') || p.startsWith(path + '\\')) {
          current.delete(p);
        }
      }
    } else {
      current.add(path);
    }
    this.expandedNodes.set(current);
  }
}
```


### `libs\contexter\ui\src\lib\file-tree\file-tree.component.spec.ts`
```
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { FileTreeComponent } from './file-tree.component';
import { FileNode } from '@org/contexter-shared';

describe('FileTreeComponent', () => {
  let component: FileTreeComponent;

  beforeEach(() => {
    component = new FileTreeComponent();
  });

  describe('toggleExpand', () => {
    it('should emit expandToggled', () => {
      const node: FileNode = { name: 'src', path: './src', isDirectory: true, children: [] };
      const spy = vi.spyOn(component.expandToggled, 'emit');
      
      component.toggleExpand(node);
      
      expect(spy).toHaveBeenCalledWith(node);
    });

    it('should also emit folderToggled if the node has no children loaded', () => {
      const node: FileNode = { name: 'src', path: './src', isDirectory: true };
      const spy = vi.spyOn(component.folderToggled, 'emit');
      
      component.toggleExpand(node);
      
      expect(spy).toHaveBeenCalledWith(node);
    });
  });

  describe('Selection logic', () => {
    it('should calculate isAllDirectFilesSelected correctly', () => {
      const node: FileNode = {
        name: 'src', path: './src', isDirectory: true,
        children: [
          { name: 'a.ts', path: './src/a.ts', isDirectory: false },
          { name: 'b.ts', path: './src/b.ts', isDirectory: false }
        ]
      };
      
      component.selectedFiles = new Set(['./src/a.ts']);
      expect(component.isAllDirectFilesSelected(node)).toBe(false);
      
      component.selectedFiles = new Set(['./src/a.ts', './src/b.ts']);
      expect(component.isAllDirectFilesSelected(node)).toBe(true);
    });
  });
});
```


### `libs\contexter\ui\src\lib\output-overlay\output-overlay.component.spec.ts`
```
import { describe, it, expect, beforeEach } from 'vitest';
import { OutputOverlayComponent } from './output-overlay.component';

describe('OutputOverlayComponent', () => {
  let component: OutputOverlayComponent;

  beforeEach(() => {
    component = new OutputOverlayComponent();
  });

  describe('willOverwrite', () => {
    it('should return true if the default filename matches an existing file', () => {
      component.defaultFilename = 'context_001.md';
      component.existingFiles = ['context_001.md', 'context_002.md'];
      
      expect(component.willOverwrite()).toBe(true);
    });

    it('should return true if the user-edited custom filename matches an existing file', () => {
      component.defaultFilename = 'context_001.md';
      component.existingFiles = ['custom.md'];
      
      // Simulate user focusing and typing
      component.onFilenameFocus(); 
      component.customFilename = 'custom.md ';
      
      expect(component.willOverwrite()).toBe(true);
    });

    it('should return false if the filename is unique', () => {
      component.defaultFilename = 'context_001.md';
      component.existingFiles = ['context_002.md'];
      
      expect(component.willOverwrite()).toBe(false);
    });
  });
});
```


### `libs\contexter\feature\eslint.config.mjs`
```
import nx from '@nx/eslint-plugin';
import baseConfig from '../../../eslint.config.mjs';

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


### `libs\contexter\feature\project.json`
```
{
  "name": "contexter-feature",
  "$schema": "../../../node_modules/nx/schemas/project-schema.json",
  "sourceRoot": "libs/contexter/feature/src",
  "prefix": "lib",
  "projectType": "library",
  "tags": ["scope:contexter"],
  "targets": {
    "lint": {
      "executor": "@nx/eslint:lint"
    }
  }
}

```


### `libs\contexter\feature\README.md`
```
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
```


### `libs\contexter\feature\tsconfig.json`
```
{
  "extends": "../../../tsconfig.base.json",
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


### `libs\contexter\feature\tsconfig.lib.json`
```
{
  "extends": "./tsconfig.json",
  "compilerOptions": {
    "outDir": "../../../dist/out-tsc",
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


### `libs\contexter\feature\tsconfig.spec.json`
```
{
  "extends": "./tsconfig.json",
  "compilerOptions": {
    "outDir": "../../../dist/out-tsc",
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


### `libs\contexter\feature\vite.config.mts`
```
/// <reference types='vitest' />
import { defineConfig } from 'vite';
import angular from '@analogjs/vite-plugin-angular';
import { nxViteTsPaths } from '@nx/vite/plugins/nx-tsconfig-paths.plugin';
import { nxCopyAssetsPlugin } from '@nx/vite/plugins/nx-copy-assets.plugin';

export default defineConfig(() => ({
  root: __dirname,
  cacheDir: '../../../node_modules/.vite/libs/contexter/feature',
  plugins: [angular(), nxViteTsPaths(), nxCopyAssetsPlugin(['*.md'])],
  // Uncomment this if you are using workers.
  // worker: {
  //   plugins: () => [ nxViteTsPaths() ],
  // },
  test: {
    name: 'contexter-feature',
    watch: false,
    globals: true,
    environment: 'jsdom',
    include: ['{src,tests}/**/*.{test,spec}.{js,mjs,cjs,ts,mts,cts,jsx,tsx}'],
    setupFiles: ['src/test-setup.ts'],
    reporters: ['default'],
    coverage: {
      reportsDirectory: '../../../coverage/libs/contexter/feature',
      provider: 'v8' as const,
    },
  },
}));

```


### `libs\contexter\feature\src\index.ts`
```
export * from './lib/layout/layout.component';
```


### `libs\contexter\feature\src\test-setup.ts`
```
import '@angular/compiler';
import '@analogjs/vitest-angular/setup-snapshots';
import { setupTestBed } from '@analogjs/vitest-angular/setup-testbed';

setupTestBed();

```


### `libs\contexter\feature\src\lib\layout\layout.component.html`
```
<div class="h-screen w-full flex overflow-hidden bg-white text-gray-900 font-sans relative">
  
  <main class="flex-1 flex flex-col min-w-0 border-r border-gray-200 relative">
    <header class="p-4 border-b border-gray-200 bg-gray-50 flex justify-between items-center shrink-0">
      <h2 class="font-bold text-lg flex items-center gap-2">
        <span>📁</span> Workspace Explorer
      </h2>
      <button 
        class="text-sm font-medium text-blue-600 hover:text-blue-800 hover:underline transition-colors"
        (click)="workspace.refreshWorkspace()">
        Refresh
      </button>
    </header>
    
    <div class="p-4 flex-1 overflow-y-auto">
      <lib-file-tree 
        [nodes]="workspace.fileTree()" 
        [selectedFiles]="selection.selectedFiles()"
        [expandedNodes]="workspace.expandedNodes()"
        (expandToggled)="workspace.toggleNodeExpansion($event.path)"
        (fileToggled)="selection.toggleFileSelection($event)"
        (folderToggled)="workspace.loadChildren($event)"
        (directoryToggled)="selection.toggleDirectoryFiles($event)">
      </lib-file-tree>
    </div>

    <!-- Output Overlay -->
    <lib-output-overlay
      [content]="bundle.generatedBundle()"
      [defaultFilename]="selection.lastContextFilename() || bundle.nextContextFilename()"
      [suggestedNewFilename]="bundle.nextContextFilename()"
      [existingFiles]="bundle.existingContexts()"
      (copy)="copyToClipboard($event)"
      (save)="onSaveContext($event)"
      (close)="bundle.generatedBundle.set('')">
    </lib-output-overlay>

    <!-- Selection Modal -->
    <lib-selection-modal
      [mode]="selectionModalMode()"
      [defaultFilename]="selection.currentSelectionFilename() || bundle.nextSelectionFilename()"
      [suggestedNewFilename]="bundle.nextSelectionFilename()"
      [existingFiles]="bundle.existingSelections()"
      (save)="onSaveSelection($event)"
      (load)="onLoadSelection($event)"
      (close)="selectionModalMode.set(null)">
    </lib-selection-modal>
  </main>

  <aside class="w-96 flex flex-col bg-gray-50 shrink-0 z-10 shadow-[-4px_0_15px_-3px_rgba(0,0,0,0.05)] relative">
    <lib-action-panel
      class="shrink-0"
      [config]="workspace.config()"
      [selectedCount]="selection.selectedFiles().size"
      (generateContext)="bundle.generateBundle()"
      (saveSelection)="selectionModalMode.set('save')"
      (loadSelection)="selectionModalMode.set('load')"
      (clearSelection)="selection.clearSelection()">
    </lib-action-panel>

    <lib-selection-panel
      class="flex-1 overflow-hidden"
      [files]="selection.selectedFiles()"
      [missingFiles]="selection.missingFiles()"
      [currentFilename]="selection.currentSelectionFilename()"
      [hasUnsavedChanges]="selection.hasUnsavedChanges()"
      (removeFile)="selection.toggleFileSelection($event)"
      (removeMissing)="selection.removeMissingFiles()"
      (quickSave)="onQuickSaveSelection()">
    </lib-selection-panel>
  </aside>

  <lib-snackbar 
    [message]="snackbarMsg()" 
    [isVisible]="snackbarVisible()"
    [type]="snackbarType()">
  </lib-snackbar>

</div>
```


### `libs\contexter\feature\src\lib\layout\layout.component.spec.ts`
```
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { signal } from '@angular/core';
import { LayoutComponent } from './layout.component';
import { WorkspaceService, SelectionService, BundleService } from '@org/contexter-data-access';

describe('LayoutComponent', () => {
  let workspaceMock: any;
  let selectionMock: any;
  let bundleMock: any;

  beforeEach(async () => {
    workspaceMock = {
      loadConfig: vi.fn().mockResolvedValue({ always_load: ['important.md'] }),
      loadDirectory: vi.fn(),
      refreshWorkspace: vi.fn(),
      fileTree: signal([]),
      expandedNodes: signal(new Set()),
      config: signal(null)
    };

    selectionMock = {
      selectedFiles: signal(new Set(['existing.ts'])),
      missingFiles: signal(new Set()),
      validateSelection: vi.fn(),
      clearSelection: vi.fn(),
    };

    bundleMock = {
      loadDefaults: vi.fn(),
      generatedBundle: signal(''),
      nextContextFilename: signal(''),
      existingContexts: signal([]),
      nextSelectionFilename: signal(''),
      existingSelections: signal([])
    };

    await TestBed.configureTestingModule({
      imports: [LayoutComponent],
      providers: [
        { provide: WorkspaceService, useValue: workspaceMock },
        { provide: SelectionService, useValue: selectionMock },
        { provide: BundleService, useValue: bundleMock }
      ]
    }).compileComponents();
  });

  it('should auto-load files from config and merge them with existing selections on boot', async () => {
    TestBed.createComponent(LayoutComponent);
    await new Promise(process.nextTick);
    
    const selected = selectionMock.selectedFiles();
    expect(selected.has('existing.ts')).toBe(true);
    expect(selected.has('important.md')).toBe(true);
    expect(selectionMock.validateSelection).toHaveBeenCalled();
  });
});
```


### `libs\contexter\feature\src\lib\layout\layout.component.ts`
```
import { Component, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { WorkspaceService, SelectionService, BundleService } from '@org/contexter-data-access';
import { 
  FileTreeComponent, 
  ActionPanelComponent, 
  SelectionPanelComponent, 
  SnackbarComponent, 
  OutputOverlayComponent, 
  SaveContextPayload, 
  SelectionModalComponent 
} from '@org/contexter-ui';

@Component({
  selector: 'lib-layout',
  standalone: true,
  imports: [
    CommonModule, 
    FileTreeComponent, 
    ActionPanelComponent, 
    SelectionPanelComponent,
    SnackbarComponent,
    OutputOverlayComponent,
    SelectionModalComponent
  ],
  templateUrl: './layout.component.html'
})
export class LayoutComponent {
  readonly workspace = inject(WorkspaceService);
  readonly selection = inject(SelectionService);
  readonly bundle = inject(BundleService);

  snackbarMsg = signal('');
  snackbarType = signal<'success' | 'error'>('success');
  snackbarVisible = signal(false);
  private snackbarTimer: any;
  
  selectionModalMode = signal<'save' | 'load' | null>(null);

  constructor() {
    this.workspace.loadConfig().then(conf => {
      if (conf?.always_load?.length) {
        const current = new Set(this.selection.selectedFiles());
        conf.always_load.forEach(f => current.add(f));
        this.selection.selectedFiles.set(current);
        this.selection.validateSelection();
      }
    });
    this.workspace.loadDirectory();
    this.bundle.loadDefaults();
  }

  showSnackbar(message: string, type: 'success' | 'error' = 'success') {
    this.snackbarMsg.set(message);
    this.snackbarType.set(type);
    this.snackbarVisible.set(true);
    clearTimeout(this.snackbarTimer);
    this.snackbarTimer = setTimeout(() => this.snackbarVisible.set(false), 3000);
  }

  copyToClipboard(text: string) {
    navigator.clipboard.writeText(text).then(() => {
      this.showSnackbar('LLM Context copied to clipboard');
    });
  }

  async onQuickSaveSelection() {
    const currentFile = this.selection.currentSelectionFilename();
    if (!currentFile) return;
    
    try {
      const response = await this.selection.saveSelection(
        currentFile, 
        this.selection.selectionDescription(), 
        this.selection.lastContextFilename()
      );
      if (response?.success) {
        this.showSnackbar(`Quick saved: ${response.filename}`);
        await this.bundle.loadDefaults();
      }
    } catch (error) {
      console.error(error);
      this.showSnackbar('Failed to quick save selection', 'error');
    }
  }

  async onSaveSelection(payload: {filename: string}) {
    try {
      const response = await this.selection.saveSelection(
        payload.filename, 
        this.selection.selectionDescription(),
        this.selection.lastContextFilename()
      );
      if (response?.success) {
        this.showSnackbar(`Selection saved: ${response.filename}`);
        this.selectionModalMode.set(null);
        await this.bundle.loadDefaults();
      }
    } catch (error) {
      console.error(error);
      this.showSnackbar('Failed to save selection', 'error');
    }
  }

  async onLoadSelection(filename: string) {
    const success = await this.selection.loadSelection(filename);
    if (success) {
      this.showSnackbar(`Loaded ${filename}`);
      this.selectionModalMode.set(null);
    } else {
      this.showSnackbar(`Failed to load ${filename}`, 'error');
    }
  }

  async onSaveContext(payload: SaveContextPayload) {
    const content = this.bundle.generatedBundle();
    if (!content) return;
    
    try {
      const response = await this.bundle.saveContextToDisk(content, payload.filename, payload.description);
      if (response?.success) {
        this.showSnackbar(`Context saved: ${response.filename}`);
        
        // This mutates the active Selection's state, instantly lighting up the [modified] Quick Save button!
        this.selection.updateLastContextFilename(response.filename);

        await this.bundle.loadDefaults();
      }
    } catch (error) {
      console.error(error);
      this.showSnackbar('Failed to save context', 'error');
    }
  }
}
```


### `apps\contexter\contexter-ng\.postcssrc.json`
```
{
  "plugins": {
    "@tailwindcss/postcss": {}
  }
}
```


### `apps\contexter\contexter-ng\eslint.config.mjs`
```
import nx from '@nx/eslint-plugin';
import baseConfig from '../../../eslint.config.mjs';

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
          prefix: 'app',
          style: 'camelCase',
        },
      ],
      '@angular-eslint/component-selector': [
        'error',
        {
          type: 'element',
          prefix: 'app',
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


### `apps\contexter\contexter-ng\project.json`
```
{
  "name": "contexter-ng",
  "$schema": "../../../node_modules/nx/schemas/project-schema.json",
  "projectType": "application",
  "prefix": "app",
  "sourceRoot": "apps/contexter/contexter-ng/src",
  "tags": ["scope:contexter"],
  "targets": {
    "build": {
      "executor": "@angular/build:application",
      "outputs": ["{options.outputPath}"],
      "defaultConfiguration": "production",
      "options": {
        "outputPath": "dist/apps/contexter/contexter-ng",
        "browser": "apps/contexter/contexter-ng/src/main.ts",
        "tsConfig": "apps/contexter/contexter-ng/tsconfig.app.json",
        "assets": [
          {
            "glob": "**/*",
            "input": "apps/contexter/contexter-ng/public"
          }
        ],
        "styles": ["apps/contexter/contexter-ng/src/styles.css"]
      },
      "configurations": {
        "production": {
          "budgets": [
            {
              "type": "initial",
              "maximumWarning": "500kb",
              "maximumError": "1mb"
            },
            {
              "type": "anyComponentStyle",
              "maximumWarning": "4kb",
              "maximumError": "8kb"
            }
          ],
          "outputHashing": "all"
        },
        "development": {
          "optimization": false,
          "extractLicenses": false,
          "sourceMap": true
        }
      }
    },
    "serve": {
      "continuous": true,
      "executor": "@angular/build:dev-server",
      "defaultConfiguration": "development",
      "configurations": {
        "production": {
          "buildTarget": "contexter-ng:build:production"
        },
        "development": {
          "buildTarget": "contexter-ng:build:development"
        }
      }
    },
    "lint": {
      "executor": "@nx/eslint:lint"
    },
    "serve-static": {
      "continuous": true,
      "executor": "@nx/web:file-server",
      "options": {
        "buildTarget": "contexter-ng:build",
        "port": 4200,
        "staticFilePath": "dist/apps/contexter/contexter-ng/browser",
        "spa": true
      }
    }
  }
}

```


### `apps\contexter\contexter-ng\readme.md`
```
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
```


### `apps\contexter\contexter-ng\tsconfig.app.json`
```
{
  "extends": "./tsconfig.json",
  "compilerOptions": {
    "outDir": "../../../dist/out-tsc",
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


### `apps\contexter\contexter-ng\tsconfig.json`
```
{
  "extends": "../../../tsconfig.base.json",
  "compilerOptions": {
    "noImplicitOverride": true,
    "noPropertyAccessFromIndexSignature": true,
    "noImplicitReturns": true,
    "noFallthroughCasesInSwitch": true,
    "isolatedModules": true,
    "target": "es2022",
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
      "path": "./tsconfig.app.json"
    },
    {
      "path": "./tsconfig.spec.json"
    }
  ]
}

```


### `apps\contexter\contexter-ng\tsconfig.spec.json`
```
{
  "extends": "./tsconfig.json",
  "compilerOptions": {
    "outDir": "../../../dist/out-tsc",
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


### `apps\contexter\contexter-ng\vite.config.mts`
```
/// <reference types='vitest' />
import { defineConfig } from 'vite';
import angular from '@analogjs/vite-plugin-angular';
import { nxViteTsPaths } from '@nx/vite/plugins/nx-tsconfig-paths.plugin';
import { nxCopyAssetsPlugin } from '@nx/vite/plugins/nx-copy-assets.plugin';

export default defineConfig(() => ({
  root: __dirname,
  cacheDir: '../../node_modules/.vite/apps/contexter-ng',
  plugins: [angular(), nxViteTsPaths(), nxCopyAssetsPlugin(['*.md'])],
  // Uncomment this if you are using workers.
  // worker: {
  //   plugins: () => [ nxViteTsPaths() ],
  // },
  test: {
    name: 'contexter-ng',
    watch: false,
    globals: true,
    environment: 'jsdom',
    include: ['{src,tests}/**/*.{test,spec}.{js,mjs,cjs,ts,mts,cts,jsx,tsx}'],
    setupFiles: ['src/test-setup.ts'],
    reporters: ['default'],
    coverage: {
      reportsDirectory: '../../coverage/apps/contexter-ng',
      provider: 'v8' as const,
    },
  },
}));

```


### `apps\contexter\contexter-ng\src\index.html`
```
<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <title>contexter-ng</title>
    <base href="/" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <link rel="icon" type="image/x-icon" href="favicon.ico" />
  </head>
  <body>
    <app-root></app-root>
  </body>
</html>

```


### `apps\contexter\contexter-ng\src\main.ts`
```
import { bootstrapApplication } from '@angular/platform-browser';
import { appConfig } from './app/app.config';
import { App } from './app/app';

bootstrapApplication(App, appConfig).catch((err) => console.error(err));

```


### `apps\contexter\contexter-ng\src\styles.css`
```
/* You can add global styles to this file, and also import other style files */
@import "tailwindcss";
@source "../../../../libs/contexter/ui";
```


### `apps\contexter\contexter-ng\src\test-setup.ts`
```
import '@angular/compiler';
import '@analogjs/vitest-angular/setup-snapshots';
import { setupTestBed } from '@analogjs/vitest-angular/setup-testbed';

setupTestBed();

```


### `libs\contexter\ui\src\lib\selection-modal\selection-modal.component.ts`
```
import { Component, EventEmitter, Output, input, effect, computed } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'lib-selection-modal',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './selection-modal.component.html'
})
export class SelectionModalComponent {
  mode = input<'save' | 'load' | null>(null);
  defaultFilename = input<string>('');
  suggestedNewFilename = input<string>('');
  existingFiles = input<string[]>([]);

  @Output() save = new EventEmitter<{filename: string}>();
  @Output() load = new EventEmitter<string>();
  @Output() close = new EventEmitter<void>();

  customFilename = '';
  private userEdited = false;

  constructor() {
    effect(() => {
      if (this.mode() === 'save' && !this.userEdited) {
        this.customFilename = this.defaultFilename();
      }
    });
  }

  incrementedFilename = computed(() => {
    const def = this.defaultFilename();
    if (!def) return '';
    const match = def.match(/^(.*?)_?(\d*)(\.[a-zA-Z0-9]+)$/);
    if (!match) return def;
    
    const base = match[1];
    const numStr = match[2];
    const ext = match[3];
    
    if (!numStr) return `${base}_01${ext}`;
    
    const num = parseInt(numStr, 10) + 1;
    const paddedNum = num.toString().padStart(numStr.length, '0');
    return `${base}_${paddedNum}${ext}`;
  });

  get canSaveAsNew(): boolean {
    return this.existingFiles().includes(this.defaultFilename()) && this.customFilename !== this.incrementedFilename();
  }

  updateFilename(e: Event) {
    this.customFilename = (e.target as HTMLInputElement).value;
    this.userEdited = true;
  }

  onFocus() {
    if (!this.userEdited && !this.existingFiles().includes(this.defaultFilename())) {
      this.customFilename = '';
    }
    this.userEdited = true;
  }

  useIncrementedName() {
    this.customFilename = this.incrementedFilename();
    this.userEdited = true;
  }

  willOverwrite(): boolean {
    const name = (this.userEdited && this.customFilename.trim()) ? this.customFilename.trim() : this.defaultFilename();
    let finalName = name;
    if (finalName && !finalName.endsWith('.yaml')) finalName += '.yaml';
    return this.existingFiles().includes(finalName);
  }

  submitSave() {
    let filename = (this.userEdited && this.customFilename.trim()) ? this.customFilename.trim() : this.defaultFilename();
    if (filename && !filename.endsWith('.yaml')) filename += '.yaml';
    this.save.emit({ filename });
    this.userEdited = false;
  }
}
```


### `libs\contexter\ui\src\lib\selection-modal\selection-modal.component.html`
```
@if (mode()) {
  <div class="absolute inset-0 z-50 bg-black/50 flex items-center justify-center p-8 backdrop-blur-sm">
    <div class="bg-white rounded-lg shadow-xl w-full max-w-lg overflow-hidden flex flex-col">
      
      <div class="p-4 border-b border-gray-200 flex justify-between items-center bg-gray-50">
        <h3 class="font-bold text-lg">
          {{ mode() === 'save' ? 'Save Selection' : 'Load Selection' }}
        </h3>
        <button (click)="close.emit()" class="text-gray-400 hover:text-gray-700 font-bold p-1">✕</button>
      </div>

      @if (mode() === 'save') {
        <div class="p-4 flex flex-col gap-4">
          <div>
            <label class="block text-xs font-bold text-gray-700 mb-1">Filename</label>
            <input 
              type="text" 
              [value]="customFilename"
              (input)="updateFilename($event)"
              (focus)="onFocus()"
              class="w-full text-sm px-3 py-2 border border-gray-300 rounded focus:border-blue-500 focus:ring-1 focus:ring-blue-500 font-mono"
            />
            
            <div class="flex items-start justify-between mt-1 min-h-[20px]">
              @if (willOverwrite()) {
                <div class="text-xs text-orange-600 font-bold flex items-center gap-1 animate-pulse">
                  <span>⚠️</span> Overwriting existing file.
                </div>
              } @else {
                <div></div>
              }
              
              @if (canSaveAsNew) {
                <button 
                  (click)="useIncrementedName()" 
                  class="text-[10px] text-blue-600 hover:text-blue-800 hover:underline font-bold transition-colors text-right mt-0.5">
                  Increment File Name ({{ incrementedFilename() }})
                </button>
              }
            </div>
          </div>
          <button 
            (click)="submitSave()"
            class="w-full bg-blue-600 hover:bg-blue-700 text-white font-bold py-2 px-4 rounded transition-colors mt-2">
            Save to Disk
          </button>
        </div>
      } @else {
        <div class="p-4 flex-1 overflow-auto max-h-96">
          <div class="text-xs font-bold text-gray-500 mb-2 uppercase tracking-wider">Available Selections</div>
          <ul class="flex flex-col gap-2">
            @for (file of existingFiles(); track file) {
              <li>
                <button 
                  (click)="load.emit(file)"
                  class="w-full text-left px-3 py-2 rounded bg-gray-50 hover:bg-blue-50 hover:text-blue-700 border border-gray-200 transition-colors font-mono text-sm">
                  {{ file }}
                </button>
              </li>
            } @empty {
              <div class="text-sm text-gray-500 italic p-4 text-center">No saved selections found.</div>
            }
          </ul>
        </div>
      }
    </div>
  </div>
}
```
