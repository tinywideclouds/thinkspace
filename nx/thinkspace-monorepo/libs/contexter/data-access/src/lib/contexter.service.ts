import { inject, Injectable, signal } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { ContextConfig, FileNode, BundleResponse } from '@org/contexter-shared';
import { firstValueFrom } from 'rxjs';

@Injectable({ providedIn: 'root' })
export class ContexterService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = 'http://localhost:3333/api';

  readonly config = signal<ContextConfig | null>(null);
  readonly fileTree = signal<FileNode[]>([]);
  readonly selectedFiles = signal<Set<string>>(new Set());
  readonly generatedBundle = signal<string>('');

  async loadConfig() {
    const conf = await firstValueFrom(this.http.get<ContextConfig>(`${this.baseUrl}/config`));
    this.config.set(conf);
    
    const selection = new Set(this.selectedFiles());
    conf.always_load.forEach(f => selection.add(f));
    this.selectedFiles.set(selection);
    
    return conf;
  }

  async loadDirectory(dirPath?: string) {
    const targetDir = dirPath || this.config()?.root_dir || './';
    const tree = await firstValueFrom(this.http.get<FileNode[]>(`${this.baseUrl}/tree?dir=${encodeURIComponent(targetDir)}`));
    this.fileTree.set(tree);
  }

  async loadChildren(node: FileNode) {
    if (node.children) return; 

    console.time(`[UI] Fetching ${node.path}`);
    try {
      const children = await firstValueFrom(
        this.http.get<FileNode[]>(`${this.baseUrl}/tree?dir=${encodeURIComponent(node.path)}`)
      );
      
      this.fileTree.update(tree => this.updateTree(tree, node.path, children));
    } catch (error) {
      console.error(`[UI] Failed to load directory contents for ${node.path}:`, error);
    } finally {
      console.timeEnd(`[UI] Fetching ${node.path}`);
    }
  }

  private updateTree(nodes: FileNode[], targetPath: string, newChildren: FileNode[]): FileNode[] {
    return nodes.map(n => {
      if (n.path === targetPath) {
        return { ...n, children: newChildren };
      }
      if (n.children) {
        return { ...n, children: this.updateTree(n.children, targetPath, newChildren) };
      }
      return n;
    });
  }

  toggleFileSelection(filePath: string) {
    const current = new Set(this.selectedFiles());
    if (current.has(filePath)) {
      current.delete(filePath);
    } else {
      current.add(filePath);
    }
    this.selectedFiles.set(current);
  }

  toggleDirectoryFiles(node: FileNode) {
    if (!node.children) return;

    const files = node.children.filter(c => !c.isDirectory);
    if (files.length === 0) return;

    const current = new Set(this.selectedFiles());
    const allSelected = files.every(f => current.has(f.path));

    if (allSelected) {
      files.forEach(f => current.delete(f.path));
    } else {
      files.forEach(f => current.add(f.path));
    }

    this.selectedFiles.set(current);
  }

  async generateBundle() {
    const files = Array.from(this.selectedFiles());
    if (files.length === 0) return '';
    
    const response = await firstValueFrom(
      this.http.post<BundleResponse>(`${this.baseUrl}/generate`, { files })
    );
    this.generatedBundle.set(response.content);
    return response.content;
  }

  async saveSelection() {
    const files = Array.from(this.selectedFiles());
    if (files.length === 0) return null;

    return firstValueFrom(
      this.http.post<{success: boolean, filename: string}>(`${this.baseUrl}/selections`, { files })
    );
  }

  async saveContextToDisk(markdown: string) {
    if (!markdown) return null;

    return firstValueFrom(
      this.http.post<{success: boolean, filename: string}>(`${this.baseUrl}/contexts`, { markdown })
    );
  }

  loadBundle(name: string) {
    const conf = this.config();
    if (conf && conf.saved_bundles[name]) {
      const files = conf.saved_bundles[name].files;
      const current = new Set(this.selectedFiles());
      files.forEach(f => current.add(f));
      this.selectedFiles.set(current);
    }
  }
}