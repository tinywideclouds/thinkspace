import { describe, it, expect, vi, beforeEach } from 'vitest';
import * as fs from 'fs/promises';
import { getNextSequenceFilename, getExistingFiles, ensureDir } from './file-system';

vi.mock('fs/promises');

describe('File System Service', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    // Supress expected console.errors in the test output
    vi.spyOn(console, 'error').mockImplementation(() => {});
  });

  describe('getNextSequenceFilename', () => {
    it('should return 001 if the directory is empty', async () => {
      vi.mocked(fs.mkdir).mockResolvedValue(undefined);
      vi.mocked(fs.readdir).mockResolvedValue([] as any);

      const filename = await getNextSequenceFilename('./test', 'context_', '.md');
      
      expect(filename).toBe('context_001.md');
    });

    it('should calculate the next number based on existing sequence files', async () => {
      vi.mocked(fs.mkdir).mockResolvedValue(undefined);
      vi.mocked(fs.readdir).mockResolvedValue([
        'context_001.md',
        'context_005.md',
        'random_file.txt'
      ] as any);

      const filename = await getNextSequenceFilename('./test', 'context_', '.md');
      
      // Should find 5 as max, return 6 padded to 006
      expect(filename).toBe('context_006.md');
    });

    it('should fallback to 001 if the directory read fails', async () => {
      vi.mocked(fs.mkdir).mockResolvedValue(undefined);
      vi.mocked(fs.readdir).mockRejectedValue(new Error('EACCES: permission denied'));

      const filename = await getNextSequenceFilename('./forbidden', 'selection_', '.yaml');
      
      expect(filename).toBe('selection_001.yaml');
      expect(console.error).toHaveBeenCalled();
    });
  });

  describe('getExistingFiles', () => {
    it('should return an array of file names on success', async () => {
      vi.mocked(fs.mkdir).mockResolvedValue(undefined);
      vi.mocked(fs.readdir).mockResolvedValue(['fileA.txt', 'fileB.txt'] as any);

      const files = await getExistingFiles('./test');
      
      expect(files).toEqual(['fileA.txt', 'fileB.txt']);
    });

    it('should catch errors and return an empty array', async () => {
      vi.mocked(fs.readdir).mockRejectedValue(new Error('Cannot read'));

      const files = await getExistingFiles('./test');
      
      expect(files).toEqual([]);
    });
  });
});