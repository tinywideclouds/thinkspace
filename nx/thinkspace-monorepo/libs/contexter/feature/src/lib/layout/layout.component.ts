import { Component, inject, signal, computed } from '@angular/core';
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

  rootFiles = computed(() => this.workspace.fileTree().filter(n => !n.isDirectory));
  
  isAllRootFilesSelected = computed(() => {
    const files = this.rootFiles();
    if (files.length === 0) return false;
    const selected = this.selection.selectedFiles();
    return files.every(f => selected.has(f.path));
  });
  
  isSomeRootFilesSelected = computed(() => {
    const files = this.rootFiles();
    if (files.length === 0) return false;
    const selected = this.selection.selectedFiles();
    const count = files.filter(f => selected.has(f.path)).length;
    return count > 0 && count < files.length;
  });

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

  async onGenerateContext() {
    await this.bundle.generateBundle();
    const skipped = this.bundle.skippedLargeFiles();
    
    if (skipped.length > 0) {
      this.showSnackbar(
        `Skipped ${skipped.length} large file(s). Check 'Allow files > 1MB' to include them.`, 
        'error'
      );
    }
  }

  onChangeRoot(event: Event) {
    const newRoot = (event.target as HTMLInputElement).value.trim() || './';
    this.workspace.loadDirectory(newRoot);
  }

  navigateUp() {
    let current = this.workspace.currentRoot().replace(/\\/g, '/');
    if (current.endsWith('/')) current = current.slice(0, -1);
    
    if (current === '.' || current === '') {
      this.workspace.loadDirectory('..');
      return;
    }
    
    const parts = current.split('/');
    if (parts[parts.length - 1] === '..') {
      parts.push('..');
    } else {
      parts.pop();
    }
    
    const newRoot = parts.length > 0 ? parts.join('/') : './';
    this.workspace.loadDirectory(newRoot);
  }

  resetRoot() {
    this.workspace.loadDirectory(this.workspace.config()?.root_dir || './');
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
        this.selection.updateLastContextFilename(response.filename);
        await this.bundle.loadDefaults();
      }
    } catch (error) {
      console.error(error);
      this.showSnackbar('Failed to save context', 'error');
    }
  }
}