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

  toggleRootFiles() {
    const files = this.workspace.fileTree().filter(c => !c.isDirectory);
    if (files.length === 0) return;

    const current = new Set(this.selectedFiles());
    const allSelected = files.every(f => current.has(f.path));

    if (allSelected) files.forEach(f => current.delete(f.path));
    else files.forEach(f => current.add(f.path));

    this.selectedFiles.set(current);
    this.validateSelection();
    this.markChanged();
  }
}