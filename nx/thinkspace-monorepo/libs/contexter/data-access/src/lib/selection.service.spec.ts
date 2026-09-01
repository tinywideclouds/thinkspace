import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { SelectionService } from './selection.service';
import { WorkspaceService } from './workspace.service';
import { FileNode } from '@org/contexter-shared';

describe('SelectionService', () => {
  let service: SelectionService;
  let httpMock: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [
        SelectionService,
        WorkspaceService,
        provideHttpClient(),
        provideHttpClientTesting()
      ]
    });
    service = TestBed.inject(SelectionService);
    httpMock = TestBed.inject(HttpTestingController);
  });

  afterEach(() => {
    httpMock.verify();
  });

  describe('validateSelection', () => {
    it('should update missingFiles signal based on backend validation', async () => {
      service.selectedFiles.set(new Set(['good.ts', 'bad.ts']));
      
      const promise = service.validateSelection();
      
      const req = httpMock.expectOne('http://localhost:3333/api/validate');
      expect(req.request.method).toBe('POST');
      expect(req.request.body).toEqual({ files: ['good.ts', 'bad.ts'] });
      
      req.flush({ missing: ['bad.ts'] });
      await promise;
      
      expect(service.missingFiles().has('bad.ts')).toBe(true);
      expect(service.missingFiles().has('good.ts')).toBe(false);
    });
  });

  describe('toggleDirectoryFiles', () => {
    it('should select all files in a directory if none or some are selected', () => {
      const node: FileNode = {
        name: 'src', path: './src', isDirectory: true,
        children: [
          { name: 'a.ts', path: './src/a.ts', isDirectory: false },
          { name: 'b.ts', path: './src/b.ts', isDirectory: false }
        ]
      };

      service.selectedFiles.set(new Set(['./src/a.ts']));
      
      service.toggleDirectoryFiles(node);
      
      httpMock.expectOne('http://localhost:3333/api/validate').flush({ missing: [] });

      expect(service.selectedFiles().has('./src/a.ts')).toBe(true);
      expect(service.selectedFiles().has('./src/b.ts')).toBe(true);
    });

    it('should deselect all files in a directory if all are currently selected', () => {
      const node: FileNode = {
        name: 'src', path: './src', isDirectory: true,
        children: [
          { name: 'a.ts', path: './src/a.ts', isDirectory: false },
          { name: 'b.ts', path: './src/b.ts', isDirectory: false }
        ]
      };

      service.selectedFiles.set(new Set(['./src/a.ts', './src/b.ts']));
      
      service.toggleDirectoryFiles(node);
      
      // Removed httpMock.expectOne() because validateSelection returns early when the list is empty

      expect(service.selectedFiles().has('./src/a.ts')).toBe(false);
      expect(service.selectedFiles().has('./src/b.ts')).toBe(false);
    });
  });

  describe('removeMissingFiles', () => {
    it('should remove all missing files from the selected files set', () => {
      service.selectedFiles.set(new Set(['good.ts', 'bad.ts']));
      service.missingFiles.set(new Set(['bad.ts']));
      
      service.removeMissingFiles();
      
      expect(service.selectedFiles().has('good.ts')).toBe(true);
      expect(service.selectedFiles().has('bad.ts')).toBe(false);
      expect(service.missingFiles().size).toBe(0);
    });
  });
});