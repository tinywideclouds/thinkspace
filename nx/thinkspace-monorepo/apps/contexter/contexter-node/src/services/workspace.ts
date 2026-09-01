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

export async function generateBundleContent(files: string[]): Promise<string> {
  let content = '';
  for (const file of files) {
    try {
      const fileContent = await fs.readFile(file, 'utf8');
      content += `\n\n### \`${file}\`\n\`\`\`\n${fileContent}\n\`\`\`\n`;
    } catch (e) {
      content += `\n\n### \`${file}\`\n<!-- FILE NOT FOUND: ${file} -->\n`;
    }
  }
  return content;
}