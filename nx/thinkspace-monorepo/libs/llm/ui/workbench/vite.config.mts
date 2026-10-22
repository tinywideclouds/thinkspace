/// <reference types='vitest' />
import { defineConfig } from 'vite';
import angular from '@analogjs/vite-plugin-angular';

export default defineConfig(() => ({
  root: __dirname,
  cacheDir: '../../../../node_modules/.vite/libs/llm/ui/workbench',
  resolve: {
    tsconfigPaths: true,
  },
  plugins: [angular()],
  test: {
    name: 'llm-ui-workbench',
    watch: false,
    globals: true,
    environment: 'jsdom',
    include: ['{src,tests}/**/*.{test,spec}.{js,mjs,cjs,ts,mts,cts,jsx,tsx}'],
    setupFiles: ['src/test-setup.ts'],
    reporters: ['default'],
    coverage: {
      reportsDirectory: '../../../../coverage/libs/llm/ui/workbench',
      provider: 'v8' as const,
    },
  },
}));