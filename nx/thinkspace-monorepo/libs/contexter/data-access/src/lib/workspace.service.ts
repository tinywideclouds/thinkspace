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
  readonly currentRoot = signal<string>('./');

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
    this.currentRoot.set(targetDir);
    
    const tree = await firstValueFrom(
      this.http.get<FileNode[]>(`${this.baseUrl}/tree?dir=${encodeURIComponent(targetDir)}`)
    );
    this.fileTree.set(this.sortNodes(tree));
    this.expandedNodes.set(new Set());
  }

  async browseForRoot() {
    try {
      const res = await firstValueFrom(
        this.http.get<{path: string | null}>(`${this.baseUrl}/browse`)
      );
      if (res.path) {
        this.loadDirectory(res.path);
      }
    } catch (e) {
      console.error('[UI] Failed to open folder picker:', e);
    }
  }

  async refreshWorkspace() {
    const targetDir = this.currentRoot();
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