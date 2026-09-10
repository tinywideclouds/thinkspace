import { Router } from 'express';
import * as path from 'path';
import * as fs from 'fs/promises';
import * as yaml from 'js-yaml';
import { exec } from 'child_process';
import * as os from 'os';
import { ensureDir, getNextSequenceFilename, getExistingFiles } from '../services/file-system';
import { getConfig, saveConfig } from '../services/config';
import { validateFiles, generateBundleContent } from '../services/workspace';

export const apiRouter = Router();

apiRouter.get('/config', async (req, res) => {
  try {
    res.json(await getConfig());
  } catch (error) {
    res.status(500).json({ error: 'Failed to load config' });
  }
});

apiRouter.post('/config', async (req, res) => {
  try {
    await saveConfig(req.body);
    res.json({ success: true });
  } catch (error) {
    res.status(500).json({ error: 'Failed to save config' });
  }
});

apiRouter.get('/tree', async (req, res) => {
  try {
    const dir = req.query['dir'] as string || './';
    const entries = await fs.readdir(dir, { withFileTypes: true });
    const nodes = entries.map(entry => ({
      name: entry.name,
      // Normalize paths to POSIX for consistent frontend string matching
      path: path.join(dir, entry.name).replace(/\\/g, '/'),
      isDirectory: entry.isDirectory()
    }));
    res.json(nodes);
  } catch (error) {
    res.status(500).json({ error: 'Failed to read directory' });
  }
});

apiRouter.get('/defaults', async (req, res) => {
  try {
    const config = await getConfig();
    const nextContext = await getNextSequenceFilename(config.paths.contexts, 'context_', '.md');
    const nextSelection = await getNextSequenceFilename(config.paths.selections, 'selection_', '.yaml');
    const existingContexts = await getExistingFiles(config.paths.contexts);
    const existingSelections = await getExistingFiles(config.paths.selections);
    res.json({ nextContext, nextSelection, existingContexts, existingSelections });
  } catch (error) {
    res.status(500).json({ error: 'Failed to calculate default filenames' });
  }
});

apiRouter.post('/validate', async (req, res) => {
  try {
    const missing = await validateFiles(req.body.files);
    res.json({ missing });
  } catch (error) {
    res.status(500).json({ error: 'Failed to validate files' });
  }
});

apiRouter.post('/generate', async (req, res) => {
  try {
    const { files, allowLargeFiles } = req.body;
    const { content, skipped } = await generateBundleContent(files, allowLargeFiles);
    res.json({ content, count: files.length - skipped.length, skippedLargeFiles: skipped });
  } catch (error) {
    res.status(500).json({ error: 'Failed to generate payload' });
  }
});

apiRouter.get('/selections', async (req, res) => {
  try {
    const config = await getConfig();
    const files = await getExistingFiles(config.paths.selections);
    res.json(files.filter(f => f.endsWith('.yaml')));
  } catch (error) {
    res.status(500).json({ error: 'Failed to list selections' });
  }
});

apiRouter.get('/selections/:filename', async (req, res) => {
  try {
    const config = await getConfig();
    const filePath = path.join(config.paths.selections, req.params.filename);
    const content = await fs.readFile(filePath, 'utf8');
    const parsed = yaml.load(content) as any;
    res.json(parsed);
  } catch (error) {
    res.status(500).json({ error: 'Failed to load selection' });
  }
});

apiRouter.post('/selections', async (req, res) => {
  try {
    const { files, filename: customFilename, description, last_context } = req.body;
    const config = await getConfig();
    const targetDir = await ensureDir(config.paths.selections);
    
    let filename = customFilename?.trim() || await getNextSequenceFilename(targetDir, 'selection_', '.yaml');
    if (!filename.endsWith('.yaml')) filename += '.yaml';
    
    const data: any = { files };
    if (description) data.description = description;
    if (last_context) data.last_context = last_context;

    await fs.writeFile(path.join(targetDir, filename), yaml.dump(data), 'utf8');
    res.json({ success: true, filename });
  } catch (error) {
    res.status(500).json({ error: 'Failed to save selection' });
  }
});

apiRouter.post('/contexts', async (req, res) => {
  try {
    const { markdown, filename: customFilename, description } = req.body;
    const config = await getConfig();
    const targetDir = await ensureDir(config.paths.contexts);
    
    let filename = customFilename?.trim() || await getNextSequenceFilename(targetDir, 'context_', '.md');
    if (!filename.endsWith('.md')) filename += '.md';
    
    const finalContent = description ? `<!-- Description: ${description} -->\n\n${markdown}` : markdown;
    await fs.writeFile(path.join(targetDir, filename), finalContent, 'utf8');
    res.json({ success: true, filename });
  } catch (error) {
    res.status(500).json({ error: 'Failed to save context' });
  }
});

// NEW: Native OS Folder Picker
apiRouter.get('/browse', (req, res) => {
  const platform = os.platform();
  let cmd = '';

  // Use the host OS native folder selection dialog
  if (platform === 'darwin') {
    cmd = `osascript -e 'tell application (path to frontmost application as text) to set myFolder to choose folder with prompt "Select Workspace Root"' -e 'POSIX path of myFolder'`;
  } else if (platform === 'win32') {
    cmd = `powershell.exe -NoProfile -Command "Add-Type -AssemblyName System.windows.forms; $f = New-Object System.Windows.Forms.FolderBrowserDialog; [void]$f.ShowDialog(); $f.SelectedPath"`;
  } else {
    cmd = `zenity --file-selection --directory`;
  }

  exec(cmd, (error, stdout) => {
    // If the user hits "Cancel", stdout is usually empty or an error is thrown
    const result = stdout ? stdout.trim() : null;
    res.json({ path: result });
  });
});