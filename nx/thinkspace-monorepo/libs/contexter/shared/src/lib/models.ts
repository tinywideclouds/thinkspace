export interface FileNode {
  name: string;
  path: string;
  isDirectory: boolean;
  children?: FileNode[];
}

export interface BundlePreset {
  description?: string;
  files: string[];
  last_context?: string;
}

export interface ContextConfig {
  root_dir: string;
  always_load: string[];
  saved_bundles: Record<string, BundlePreset>;
  paths: {
    selections: string;
    contexts: string;
  };
  counters: {
    selections: number;
    contexts: number;
  };
}

export interface BundleRequest {
  files: string[];
}

export interface BundleResponse {
  content: string;
  count: number;
}