import { describe, it, expect, beforeEach, vi } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { signal } from '@angular/core';
import { LayoutComponent } from './layout.component';
import { WorkspaceService, SelectionService, BundleService } from '@org/contexter-data-access';

describe('LayoutComponent', () => {
  let workspaceMock: any;
  let selectionMock: any;
  let bundleMock: any;

  beforeEach(async () => {
    workspaceMock = {
      loadConfig: vi.fn().mockResolvedValue({ always_load: ['important.md'] }),
      loadDirectory: vi.fn(),
      refreshWorkspace: vi.fn(),
      fileTree: signal([]),
      expandedNodes: signal(new Set()),
      config: signal(null)
    };

    selectionMock = {
      selectedFiles: signal(new Set(['existing.ts'])),
      missingFiles: signal(new Set()),
      validateSelection: vi.fn(),
      clearSelection: vi.fn(),
    };

    bundleMock = {
      loadDefaults: vi.fn(),
      generatedBundle: signal(''),
      nextContextFilename: signal(''),
      existingContexts: signal([]),
      nextSelectionFilename: signal(''),
      existingSelections: signal([])
    };

    await TestBed.configureTestingModule({
      imports: [LayoutComponent],
      providers: [
        { provide: WorkspaceService, useValue: workspaceMock },
        { provide: SelectionService, useValue: selectionMock },
        { provide: BundleService, useValue: bundleMock }
      ]
    }).compileComponents();
  });

  it('should auto-load files from config and merge them with existing selections on boot', async () => {
    TestBed.createComponent(LayoutComponent);
    await new Promise(process.nextTick);
    
    const selected = selectionMock.selectedFiles();
    expect(selected.has('existing.ts')).toBe(true);
    expect(selected.has('important.md')).toBe(true);
    expect(selectionMock.validateSelection).toHaveBeenCalled();
  });
});