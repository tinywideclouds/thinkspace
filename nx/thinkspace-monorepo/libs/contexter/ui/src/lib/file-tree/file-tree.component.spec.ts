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