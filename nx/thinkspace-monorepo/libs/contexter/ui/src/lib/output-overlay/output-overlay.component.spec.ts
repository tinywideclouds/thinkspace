import { describe, it, expect, beforeEach } from 'vitest';
import { OutputOverlayComponent } from './output-overlay.component';

describe('OutputOverlayComponent', () => {
  let component: OutputOverlayComponent;

  beforeEach(() => {
    component = new OutputOverlayComponent();
  });

  describe('willOverwrite', () => {
    it('should return true if the default filename matches an existing file', () => {
      component.defaultFilename = 'context_001.md';
      component.existingFiles = ['context_001.md', 'context_002.md'];
      
      expect(component.willOverwrite()).toBe(true);
    });

    it('should return true if the user-edited custom filename matches an existing file', () => {
      component.defaultFilename = 'context_001.md';
      component.existingFiles = ['custom.md'];
      
      // Simulate user focusing and typing
      component.onFilenameFocus(); 
      component.customFilename = 'custom.md ';
      
      expect(component.willOverwrite()).toBe(true);
    });

    it('should return false if the filename is unique', () => {
      component.defaultFilename = 'context_001.md';
      component.existingFiles = ['context_002.md'];
      
      expect(component.willOverwrite()).toBe(false);
    });
  });
});