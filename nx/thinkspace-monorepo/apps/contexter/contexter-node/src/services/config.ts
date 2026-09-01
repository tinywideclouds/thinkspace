import * as fs from 'fs/promises';
import * as path from 'path';
import * as yaml from 'js-yaml';
import { ContextConfig } from '@org/contexter-shared';

const CONFIG_PATH = path.join(process.cwd(), 'default.yaml');

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

export async function ensureConfigExists() {
  try {
    await fs.access(CONFIG_PATH);
  } catch {
    await fs.mkdir(path.dirname(CONFIG_PATH), { recursive: true });
    await fs.writeFile(CONFIG_PATH, yaml.dump(DEFAULT_CONFIG), 'utf8');
  }
}

export async function getConfig(): Promise<ContextConfig> {
  await ensureConfigExists();
  const file = await fs.readFile(CONFIG_PATH, 'utf8');
  return yaml.load(file) as ContextConfig;
}

export async function saveConfig(config: ContextConfig): Promise<void> {
  await ensureConfigExists();
  await fs.writeFile(CONFIG_PATH, yaml.dump(config), 'utf8');
}