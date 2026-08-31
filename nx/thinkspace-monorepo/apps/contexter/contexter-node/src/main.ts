import express from 'express';
import cors from 'cors';
import * as fs from 'fs/promises';
import * as path from 'path';
import * as yaml from 'js-yaml';
import { ContextConfig, FileNode, BundleRequest } from '@org/contexter-shared';

const app = express();
app.use(cors());
app.use(express.json());

const CONFIG_PATH = path.resolve(process.cwd(), 'apps/contexter/configs/default.yaml');

const DEFAULT_CONFIG: ContextConfig = {
  root_dir: './',
  always_load: [],
  saved_bundles: {},
  paths: {
    selections: 'apps/contexter/bundles',
    contexts: 'apps/contexter/contexts'
  },
  counters: {
    selections: 1,
    contexts: 1
  }
};

async function ensureConfigExists() {
  try {
    await fs.access(CONFIG_PATH);
  } catch {
    await fs.mkdir(path.dirname(CONFIG_PATH), { recursive: true });
    await fs.writeFile(CONFIG_PATH, yaml.dump(DEFAULT_CONFIG), 'utf8');
  }
}

async function getConfig(): Promise<ContextConfig> {
  await ensureConfigExists();
  const file = await fs.readFile(CONFIG_PATH, 'utf8');
  const config = yaml.load(file) as Partial<ContextConfig>;
  
  // Safely merge saved config with defaults (in case of old config files)
  return {
    ...DEFAULT_CONFIG,
    ...config,
    paths: { ...DEFAULT_CONFIG.paths, ...(config.paths || {}) },
    counters: { ...DEFAULT_CONFIG.counters, ...(config.counters || {}) }
  };
}

async function saveConfig(config: ContextConfig) {
  await fs.writeFile(CONFIG_PATH, yaml.dump(config), 'utf8');
}

async function ensureDir(dirPath: string) {
  const absolutePath = path.resolve(process.cwd(), dirPath);
  try {
    await fs.access(absolutePath);
  } catch {
    await fs.mkdir(absolutePath, { recursive: true });
  }
  return absolutePath;
}

// --- CONFIG ENDPOINTS ---

app.get('/api/config', async (req, res) => {
  try {
    const config = await getConfig();
    res.json(config);
  } catch (error) {
    res.status(500).json({ error: 'Failed to load configuration' });
  }
});

app.post('/api/config', async (req, res) => {
  try {
    const newConfig = req.body as ContextConfig;
    await saveConfig(newConfig);
    res.json({ success: true });
  } catch (error) {
    res.status(500).json({ error: 'Failed to save configuration' });
  }
});

// --- EXPLORER ENDPOINT ---

app.get('/api/tree', async (req, res) => {
  const startTime = performance.now();
  const targetDir = (req.query.dir as string) || './';
  
  console.log(`\n[API] GET /api/tree?dir=${targetDir}`);
  
  try {
    const absoluteTarget = path.resolve(targetDir);
    const workspaceRoot = path.resolve('./'); 

    console.log(`[API] Resolving absolute path: ${absoluteTarget}`);

    const entries = await fs.readdir(absoluteTarget, { withFileTypes: true });
    const nodes: FileNode[] = [];

    for (const entry of entries) {
      if (['node_modules', '.git', 'dist', 'tmp', 'out-tsc', '.nx'].includes(entry.name)) continue;

      const fullPath = path.join(absoluteTarget, entry.name);
      const isDirectory = entry.isDirectory();

      nodes.push({
        name: entry.name,
        path: path.relative(workspaceRoot, fullPath).replace(/\\/g, '/'),
        isDirectory,
      });
    }

    nodes.sort((a, b) => {
      if (a.isDirectory === b.isDirectory) return a.name.localeCompare(b.name);
      return a.isDirectory ? -1 : 1;
    });

    const duration = performance.now() - startTime;
    console.log(`[API] Success: Returned ${nodes.length} items in ${duration.toFixed(2)}ms`);
    
    res.json(nodes);
  } catch (error: any) {
    console.error(`[API] Error reading directory:`, error.message);
    res.status(500).json({ error: 'Failed to read directory tree' });
  }
});

// --- PAYLOAD GENERATION ENDPOINT ---

app.post('/api/generate', async (req, res) => {
  try {
    const { files } = req.body as BundleRequest;
    let bundleOutput = '';

    for (const filePath of files) {
      try {
        const absolutePath = path.resolve(filePath);
        const content = await fs.readFile(absolutePath, 'utf8');
        const ext = path.extname(filePath).replace('.', '') || 'text';
        
        bundleOutput += `### \`${filePath}\`\n\`\`\`${ext}\n${content}\n\`\`\`\n\n`;
      } catch (err: any) {
        bundleOutput += `### \`${filePath}\`\n// Error reading file: ${err.message}\n\n`;
      }
    }

    res.json({ content: bundleOutput, count: files.length });
  } catch (error) {
    res.status(500).json({ error: 'Failed to generate context' });
  }
});

// --- POINTER ENDPOINT: Save Selection ---

app.post('/api/selections', async (req, res) => {
  try {
    const { files } = req.body as BundleRequest;
    const config = await getConfig();
    
    const count = config.counters.selections.toString().padStart(3, '0');
    const filename = `selection_${count}.yaml`;
    
    const targetDir = await ensureDir(config.paths.selections);
    const filePath = path.join(targetDir, filename);
    
    await fs.writeFile(filePath, yaml.dump({ files }), 'utf8');
    
    config.counters.selections++;
    await saveConfig(config);
    
    res.json({ success: true, filename });
  } catch (error) {
    console.error('[API] Error saving selection:', error);
    res.status(500).json({ error: 'Failed to save selection' });
  }
});

// --- PAYLOAD ENDPOINT: Save Context to Disk ---

app.post('/api/contexts', async (req, res) => {
  try {
    const { markdown } = req.body;
    const config = await getConfig();
    
    const count = config.counters.contexts.toString().padStart(3, '0');
    const filename = `context_${count}.md`;
    
    const targetDir = await ensureDir(config.paths.contexts);
    const filePath = path.join(targetDir, filename);
    
    await fs.writeFile(filePath, markdown, 'utf8');
    
    config.counters.contexts++;
    await saveConfig(config);
    
    res.json({ success: true, filename });
  } catch (error) {
    console.error('[API] Error saving context:', error);
    res.status(500).json({ error: 'Failed to save context' });
  }
});

const port = process.env.PORT || 3333;
app.listen(port, () => {
  console.log(`Context API listening at http://localhost:${port}/api`);
});