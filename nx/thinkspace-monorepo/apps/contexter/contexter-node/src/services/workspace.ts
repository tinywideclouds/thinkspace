import * as fs from 'fs/promises';

export async function validateFiles(files: string[]): Promise<string[]> {
  const missing = [];
  for (const file of files) {
    try {
      await fs.stat(file);
    } catch {
      missing.push(file);
    }
  }
  return missing;
}

export async function generateBundleContent(files: string[], allowLargeFiles = false): Promise<{content: string, skipped: string[]}> {
  let content = '';
  const skipped: string[] = [];
  
  for (const file of files) {
    try {
      const stats = await fs.stat(file);
      
      if (!allowLargeFiles && stats.size > 1024 * 1024) {
        skipped.push(file);
        continue;
      }
      
      const fileContent = await fs.readFile(file, 'utf8');
      content += `\n\n### \`${file}\`\n\`\`\`\n${fileContent}\n\`\`\`\n`;
    } catch (e) {
      content += `\n\n### \`${file}\`\n<!-- FILE NOT FOUND OR UNREADABLE: ${file} -->\n`;
    }
  }
  return { content, skipped };
}