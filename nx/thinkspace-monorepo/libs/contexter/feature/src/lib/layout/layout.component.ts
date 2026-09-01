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