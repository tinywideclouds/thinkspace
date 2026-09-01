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