import * as fs from 'fs/promises';

export async function ensureDir(dirPath: string): Promise<string> {
  await fs.mkdir(dirPath, { recursive: true });
  return dirPath;
}

// Add this to your existing file-system.ts exports
export async function getExistingFiles(dirPath: string): Promise<string[]> {
  try {
    await ensureDir(dirPath);
    return await fs.readdir(dirPath);
  } catch {
    return [];
  }
}
/**
 * Scans a directory for files matching prefix_XXX.ext and returns the next logical filename.
 * Example: getNextSequenceFilename('./contexts', 'context_', '.md') -> 'context_003.md'
 */
export async function getNextSequenceFilename(
  dirPath: string, 
  prefix: string, 
  extension: string
): Promise<string> {
  await ensureDir(dirPath);
  
  try {
    const files = await fs.readdir(dirPath);
    let maxCount = 0;
    
    // Regex to match exact pattern: e.g., ^context_(\d+)\.md$
    const regex = new RegExp(`^${prefix}(\\d+)${extension.replace('.', '\\.')}$`);
    
    for (const file of files) {
      const match = file.match(regex);
      if (match) {
        const num = parseInt(match[1], 10);
        if (num > maxCount) {
          maxCount = num;
        }
      }
    }
    
    const nextCount = (maxCount + 1).toString().padStart(3, '0');
    return `${prefix}${nextCount}${extension}`;
  } catch (error) {
    console.error(`[FS] Error reading directory ${dirPath}:`, error);
    // Safe fallback if directory is unreadable
    return `${prefix}001${extension}`;
  }
}