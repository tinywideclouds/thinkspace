import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { WorkspaceStateService } from './workspace-state.service';
import { ThinkspaceStateService } from './thinkspace-state.service';
import { signal } from '@angular/core';
import { describe, beforeEach, afterEach, it, expect } from 'vitest';

describe('WorkspaceStateService', () => {
  let service: WorkspaceStateService;
  let httpTestingController: HttpTestingController;
  let mockThinkspace: any;

  beforeEach(() => {
    mockThinkspace = {
      activeSpaceId: signal('golang')
    };

    TestBed.configureTestingModule({
      providers: [
        WorkspaceStateService,
        provideHttpClient(),
        provideHttpClientTesting(),
        { provide: ThinkspaceStateService, useValue: mockThinkspace }
      ]
    });
    
    service = TestBed.inject(WorkspaceStateService);
    httpTestingController = TestBed.inject(HttpTestingController);
  });

  afterEach(() => {
    httpTestingController.verify();
  });

  it('should fetch file content passing the path as a query param', async () => {
    const promise = service.getFileContent('src/main.go', 'hash123');
    
    const request = httpTestingController.expectOne(
      req => req.url === '/api/spaces/golang/files/hash123' && req.params.get('path') === 'src/main.go'
    );
    expect(request.request.method).toBe('GET');
    expect(request.request.responseType).toBe('text');
    
    request.flush('package main\n\nfunc main() {}');
    
    const content = await promise;
    expect(content).toBe('package main\n\nfunc main() {}');
  });

  it('should throw an error if there is no active space', async () => {
    mockThinkspace.activeSpaceId.set(null);
    
    await expect(service.getFileContent('src/main.go', 'hash123')).rejects.toThrow('No active space selected.');
  });
});