import { describe, it, expect, vi, beforeEach } from 'vitest';
import * as fs from 'fs/promises';
import { validateFiles, generateBundleContent } from './workspace';

vi.mock('fs/promises');

describe('Workspace Service', () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  describe('validateFiles', () => {
    it('should return an empty array when all files exist', async () => {
      vi.mocked(fs.stat).mockResolvedValue({} as any);

      const missing = await validateFiles(['file1.ts', 'file2.ts']);
      
      expect(missing).toEqual([]);
      expect(fs.stat).toHaveBeenCalledTimes(2);
    });

    it('should return an array of missing file paths when stat fails', async () => {
      vi.mocked(fs.stat).mockImplementation(async (filePath) => {
        if (filePath === 'missing.ts') throw new Error('ENOENT');
        return {} as any;
      });

      const missing = await validateFiles(['exists.ts', 'missing.ts']);
      
      expect(missing).toEqual(['missing.ts']);
    });
  });

  describe('generateBundleContent', () => {
    it('should wrap successfully read file content in markdown blocks', async () => {
      vi.mocked(fs.readFile).mockResolvedValue('console.log("hello");');

      const content = await generateBundleContent(['script.js']);
      
      expect(content).toContain('### `script.js`');
      expect(content).toContain('```\nconsole.log("hello");\n```');
    });

    it('should inject a FILE NOT FOUND comment when a file fails to read', async () => {
      vi.mocked(fs.readFile).mockRejectedValue(new Error('ENOENT'));

      const content = await generateBundleContent(['dead-file.js']);
      
      expect(content).toContain('### `dead-file.js`');
      expect(content).toContain('<!-- FILE NOT FOUND: dead-file.js -->');
    });

    it('should handle a mix of successful and failed files', async () => {
      vi.mocked(fs.readFile).mockImplementation(async (filePath) => {
        if (filePath === 'good.js') return 'const a = 1;';
        throw new Error('ENOENT');
      });

      const content = await generateBundleContent(['good.js', 'bad.js']);
      
      expect(content).toContain('```\nconst a = 1;\n```');
      expect(content).toContain('<!-- FILE NOT FOUND: bad.js -->');
    });
  });
});