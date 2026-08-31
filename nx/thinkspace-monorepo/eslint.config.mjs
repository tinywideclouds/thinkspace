import nx from '@nx/eslint-plugin';

export default [
  ...nx.configs['flat/base'],
  ...nx.configs['flat/typescript'],
  ...nx.configs['flat/javascript'],
  {
    ignores: [
      '**/dist',
      '**/vite.config.*.timestamp*',
      '**/vitest.config.*.timestamp*',
    ],
  },
  {
    files: ['**/*.ts', '**/*.tsx', '**/*.js', '**/*.jsx'],
    rules: {
      '@nx/enforce-module-boundaries': [
        'error',
        {
          enforceBuildableLibDependency: true,
          allow: ['^.*/eslint(\\.base)?\\.config\\.[cm]?[jt]s$'],
          depConstraints: [
            {
              sourceTag: 'scope:contexter',
              onlyDependOnLibsWithTags: ['scope:contexter'],
            },
            {
              sourceTag: 'type:facade',
              onlyDependOnLibsWithTags: [
                'type:protos',
                'type:facade',
                'scope:shared',
              ],
            },
            // The protos should be isolated and not depend on anything else (or just shared)
            {
              sourceTag: 'type:protos',
              onlyDependOnLibsWithTags: ['type:protos'],
            },
            // Standard rule for any other LLM libraries (like transport) so they CANNOT see protos
            {
              sourceTag: 'scope:llm',
              onlyDependOnLibsWithTags: ['scope:llm', 'scope:shared'],
              // Optionally exclude 'type:protos' here if you want to be extra safe,
              // but the type-based rules above usually supersede effectively if applied correctly.
            },
            {
              sourceTag: 'scope:shared',
              onlyDependOnLibsWithTags: ['scope:shared'],
            },
            {
              sourceTag: 'scope:shop',
              onlyDependOnLibsWithTags: ['scope:shop', 'scope:shared'],
            },
            {
              sourceTag: 'scope:api',
              onlyDependOnLibsWithTags: ['scope:api', 'scope:shared'],
            },
            {
              sourceTag: 'type:data',
              onlyDependOnLibsWithTags: ['type:data'],
            },
          ],
        },
      ],
    },
  },
  {
    files: [
      '**/*.ts',
      '**/*.tsx',
      '**/*.cts',
      '**/*.mts',
      '**/*.js',
      '**/*.jsx',
      '**/*.cjs',
      '**/*.mjs',
    ],
    // Override or add rules here
    rules: {},
  },
];
