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