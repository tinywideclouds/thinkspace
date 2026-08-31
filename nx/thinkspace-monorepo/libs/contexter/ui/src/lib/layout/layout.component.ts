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