/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_USE_MOCK: string;
  // add more env variables here later...
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}