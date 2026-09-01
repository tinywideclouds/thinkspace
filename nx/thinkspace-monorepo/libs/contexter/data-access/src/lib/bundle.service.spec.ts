import { TestBed } from '@angular/core/testing';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { BundleService } from './bundle.service';
import { SelectionService } from './selection.service';
import { WorkspaceService } from './workspace.service';

describe('BundleService', () => {
  let service: BundleService;
  let selectionService: SelectionService;
  let httpMock: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [
        BundleService,
        SelectionService,
        WorkspaceService,
        provideHttpClient(),
        provideHttpClientTesting()
      ]
    });
    service = TestBed.inject(BundleService);
    selectionService = TestBed.inject(SelectionService);
    httpMock = TestBed.inject(HttpTestingController);
  });

  afterEach(() => {
    httpMock.verify();
  });

  describe('generateBundle', () => {
    it('should return empty string and not make HTTP call if no files are selected', async () => {
      selectionService.selectedFiles.set(new Set());
      
      const content = await service.generateBundle();
      
      expect(content).toBe('');
      httpMock.expectNone('http://localhost:3333/api/generate');
    });

    it('should post selected files and set the generated bundle signal', async () => {
      selectionService.selectedFiles.set(new Set(['./main.ts']));
      
      const promise = service.generateBundle();
      
      const req = httpMock.expectOne('http://localhost:3333/api/generate');
      expect(req.request.body).toEqual({ files: ['./main.ts'] });
      
      req.flush({ content: 'mock content', count: 1 });
      const result = await promise;
      
      expect(result).toBe('mock content');
      expect(service.generatedBundle()).toBe('mock content');
    });
  });
});