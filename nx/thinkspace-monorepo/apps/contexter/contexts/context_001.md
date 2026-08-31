### `libs/contexter/ui/src/lib/action-panel/action-panel.component.html`
```html
<div class="p-4 bg-white border-b border-gray-200">
  <div class="flex flex-col gap-3">
    
    <!-- Payload Action -->
    <button 
      (click)="generateContext.emit()"
      [disabled]="selectedCount === 0"
      class="w-full flex items-center justify-center gap-2 bg-blue-600 hover:bg-blue-700 disabled:bg-gray-300 text-white font-bold py-2 px-4 rounded transition-colors">
      <span>⚡</span> Generate LLM Context
    </button>
    
    <hr class="border-gray-100 my-1" />

    <!-- Pointer Actions -->
    <div class="flex gap-2">
      <button 
        (click)="saveSelection.emit()"
        [disabled]="selectedCount === 0"
        class="flex-1 bg-gray-100 hover:bg-gray-200 disabled:bg-gray-50 disabled:text-gray-400 text-gray-700 font-medium py-2 px-3 rounded transition-colors text-sm">
        Save Selection
      </button>
      
      <button 
        (click)="loadSelection.emit('temp_placeholder')"
        class="flex-1 bg-gray-100 hover:bg-gray-200 text-gray-700 font-medium py-2 px-3 rounded transition-colors text-sm">
        Load Selection
      </button>
    </div>

    <div class="text-xs text-gray-500 text-center mt-1">
      {{ selectedCount }} files selected
    </div>
  </div>
</div>
```

### `libs/contexter/ui/src/lib/action-panel/action-panel.component.ts`
```ts
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
}
```

### `libs/contexter/ui/src/lib/file-tree/file-tree.component.html`
```html
<ul class="list-none pl-4 border-l border-gray-200">
  @for (node of nodes; track node.path) {
    <li class="my-1">
      @if (node.isDirectory) {
        <div 
          class="cursor-pointer font-medium text-gray-700 flex items-center gap-2 p-1.5 rounded hover:bg-blue-50 hover:text-blue-700 transition-colors select-none"
          (click)="toggleExpand(node)">
          <span class="text-lg leading-none">{{ expanded.has(node.path) ? '📂' : '📁' }}</span>
          
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
          
          @if (expanded.has(node.path) && !node.children) {
            <span class="text-xs text-blue-500 font-semibold animate-pulse ml-2">Loading...</span>
          }
        </div>
        
        @if (expanded.has(node.path) && node.children) {
          <lib-file-tree 
            [nodes]="node.children" 
            [selectedFiles]="selectedFiles"
            (fileToggled)="fileToggled.emit($event)"
            (folderToggled)="folderToggled.emit($event)"
            (directoryToggled)="directoryToggled.emit($event)">
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

### `libs/contexter/ui/src/lib/file-tree/file-tree.component.ts`
```ts
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
  
  @Output() fileToggled = new EventEmitter<string>();
  @Output() folderToggled = new EventEmitter<FileNode>();
  @Output() directoryToggled = new EventEmitter<FileNode>();

  expanded = new Set<string>();

  toggleExpand(node: FileNode) {
    if (this.expanded.has(node.path)) {
      this.expanded.delete(node.path);
    } else {
      this.expanded.add(node.path);
      if (!node.children) {
        this.folderToggled.emit(node);
      }
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

### `libs/contexter/ui/src/lib/layout/layout.component.html`
```html
<div class="h-screen w-full flex overflow-hidden bg-white text-gray-900 font-sans relative">
  
  <main class="flex-1 flex flex-col min-w-0 border-r border-gray-200 relative">
    <header class="p-4 border-b border-gray-200 bg-gray-50 flex justify-between items-center shrink-0">
      <h2 class="font-bold text-lg flex items-center gap-2">
        <span>📁</span> Workspace Explorer
      </h2>
      <button 
        class="text-sm font-medium text-blue-600 hover:text-blue-800 hover:underline transition-colors"
        (click)="service.loadDirectory()">
        Refresh
      </button>
    </header>
    
    <div class="p-4 flex-1 overflow-y-auto">
      <lib-file-tree 
        [nodes]="service.fileTree()" 
        [selectedFiles]="service.selectedFiles()"
        (fileToggled)="service.toggleFileSelection($event)"
        (folderToggled)="service.loadChildren($event)"
        (directoryToggled)="service.toggleDirectoryFiles($event)">
      </lib-file-tree>
    </div>

    <lib-output-overlay
      [content]="service.generatedBundle()"
      (copy)="copyToClipboard($event)"
      (save)="onSaveContext()"
      (close)="service.generatedBundle.set('')">
    </lib-output-overlay>
  </main>

  <aside class="w-96 flex flex-col bg-gray-50 shrink-0 z-10 shadow-[-4px_0_15px_-3px_rgba(0,0,0,0.05)] relative">
    <lib-action-panel
      class="shrink-0"
      [config]="service.config()"
      [selectedCount]="service.selectedFiles().size"
      (generateContext)="service.generateBundle()"
      (saveSelection)="onSaveSelection()"
      (loadSelection)="service.loadBundle($event)">
    </lib-action-panel>

    <lib-selection-panel
      class="flex-1 overflow-hidden"
      [files]="service.selectedFiles()"
      (removeFile)="service.toggleFileSelection($event)">
    </lib-selection-panel>
  </aside>

  <lib-snackbar 
    [message]="snackbarMsg()" 
    [isVisible]="snackbarVisible()">
  </lib-snackbar>

</div>
```

### `libs/contexter/ui/src/lib/layout/layout.component.ts`
```ts
import { Component, inject, OnInit, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ContexterService } from '@org/contexter-data-access';
import { FileTreeComponent } from '../file-tree/file-tree.component';
import { ActionPanelComponent } from '../action-panel/action-panel.component';
import { SelectionPanelComponent } from '../selection-panel/selection-panel.component';
import { SnackbarComponent } from '../snackbar/snackbar.component';
import { OutputOverlayComponent } from '../output-overlay/output-overlay.component';

@Component({
  selector: 'lib-layout',
  standalone: true,
  imports: [
    CommonModule, 
    FileTreeComponent, 
    ActionPanelComponent, 
    SelectionPanelComponent,
    SnackbarComponent,
    OutputOverlayComponent
  ],
  templateUrl: './layout.component.html'
})
export class LayoutComponent implements OnInit {
  readonly service = inject(ContexterService);

  snackbarMsg = signal('');
  snackbarVisible = signal(false);
  private snackbarTimer: any;

  ngOnInit() {
    this.service.loadConfig();
    this.service.loadDirectory();
  }

  showSnackbar(message: string) {
    this.snackbarMsg.set(message);
    this.snackbarVisible.set(true);
    
    clearTimeout(this.snackbarTimer);
    this.snackbarTimer = setTimeout(() => {
      this.snackbarVisible.set(false);
    }, 3000);
  }

  copyToClipboard(text: string) {
    navigator.clipboard.writeText(text).then(() => {
      this.showSnackbar('LLM Context copied to clipboard');
    });
  }

  async onSaveSelection() {
    try {
      const response = await this.service.saveSelection();
      if (response?.success) {
        this.showSnackbar(`Selection saved: ${response.filename}`);
      }
    } catch (error) {
      console.error(error);
      this.showSnackbar('Failed to save selection');
    }
  }

  async onSaveContext() {
    const content = this.service.generatedBundle();
    if (!content) return;
    
    try {
      const response = await this.service.saveContextToDisk(content);
      if (response?.success) {
        this.showSnackbar(`Context saved: ${response.filename}`);
      }
    } catch (error) {
      console.error(error);
      this.showSnackbar('Failed to save context');
    }
  }
}
```

### `libs/contexter/ui/src/lib/output-overlay/output-overlay.component.ts`
```ts
import { Component, EventEmitter, Input, Output } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'lib-output-overlay',
  standalone: true,
  imports: [CommonModule],
  template: `
    @if (content) {
      <div class="absolute inset-0 z-50 bg-black/50 flex items-center justify-center p-8 backdrop-blur-sm">
        <div class="bg-white rounded-lg shadow-2xl w-full max-w-5xl h-full max-h-[80vh] flex flex-col">
          <div class="p-4 border-b border-gray-200 flex justify-between items-center bg-gray-50 rounded-t-lg shrink-0">
            <h3 class="font-bold text-lg">LLM Context</h3>
            <div class="flex gap-4">
              <button 
                (click)="copy.emit(content)" 
                class="text-sm font-medium text-blue-600 hover:underline">
                Copy to Clipboard
              </button>
              <button 
                (click)="save.emit()" 
                class="text-sm font-medium text-blue-600 hover:underline">
                Save to Disk
              </button>
              <button 
                (click)="close.emit()" 
                class="text-sm font-bold text-gray-500 hover:text-gray-800">
                Close
              </button>
            </div>
          </div>
          <div class="flex-1 overflow-auto p-4 bg-gray-900 rounded-b-lg">
            <pre class="text-gray-100 text-sm whitespace-pre-wrap font-mono">{{ content }}</pre>
          </div>
        </div>
      </div>
    }
  `
})
export class OutputOverlayComponent {
  @Input() content = '';
  @Output() copy = new EventEmitter<string>();
  @Output() save = new EventEmitter<void>();
  @Output() close = new EventEmitter<void>();
}
```

### `libs/contexter/ui/src/lib/selection-panel/selection-panel.component.html`
```html
<div class="flex flex-col h-full bg-gray-50">
  <div class="p-4 border-b border-gray-200 bg-gray-100">
    <h3 class="text-xs font-bold text-gray-500 uppercase tracking-wider">Selected Files</h3>
  </div>
  
  <ul class="flex-1 overflow-y-auto p-4 space-y-2 text-sm">
    @for (file of files; track file) {
      <li class="flex items-start gap-2 group">
        <button 
          (click)="removeFile.emit(file)" 
          class="text-red-400 hover:text-red-600 font-bold px-1 rounded hover:bg-red-50 transition-colors shrink-0">
          ×
        </button>
        <code class="bg-white border border-gray-200 px-1.5 py-0.5 rounded text-gray-700 break-all">{{ file }}</code>
      </li>
    } @empty {
      <li class="text-gray-400 italic text-center mt-10">No files selected.</li>
    }
  </ul>
</div>
```

### `libs/contexter/ui/src/lib/selection-panel/selection-panel.component.ts`
```ts
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
  @Output() removeFile = new EventEmitter<string>();
}
```

### `libs/contexter/ui/src/lib/snackbar/snackbar.component.ts`
```ts
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
      <span class="text-green-400">✓</span>
      {{ message }}
    </div>
  `
})
export class SnackbarComponent {
  @Input() message = '';
  @Input() isVisible = false;
}
```

