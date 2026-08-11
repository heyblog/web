import { createModuleEslintConfig } from '@heyblog/configs/shared/eslint';

import tsParser from '@typescript-eslint/parser';
import astro from 'eslint-plugin-astro';
import betterTailwindcss from 'eslint-plugin-better-tailwindcss';
import { getDefaultSelectors } from 'eslint-plugin-better-tailwindcss/defaults';
import svelte from 'eslint-plugin-svelte';

export default [
  ...createModuleEslintConfig({
    additionalConfigs: [
      ...astro.configs['flat/recommended'],
      ...svelte.configs['flat/recommended'],
      ...svelte.configs['flat/prettier'],
      {
        files: ['**/*.svelte', '**/*.svelte.js', '**/*.svelte.ts'],
        languageOptions: {
          parserOptions: {
            extraFileExtensions: ['.svelte'],
            parser: tsParser,
          },
        },
        rules: {
          'svelte/prefer-svelte-reactivity': 'warn',
          'svelte/require-each-key': 'warn',
        },
      },
    ],
    additionalExtensions: ['astro', 'svelte'],
    moduleDir: import.meta.dirname,
    runtime: 'web',
  }),
  betterTailwindcss.configs.recommended,
  {
    settings: {
      'better-tailwindcss': {
        cwd: import.meta.dirname,
        entryPoint: 'src/styles/global.css',
        selectors: [
          ...getDefaultSelectors(),
          {
            kind: 'variable',
            name: '^.+Classes?$',
            match: [{ type: 'strings' }, { type: 'objectValues' }],
          },
        ],
      },
    },
    rules: {
      'better-tailwindcss/enforce-consistent-class-order': ['error', { order: 'official' }],
      'better-tailwindcss/enforce-consistent-line-wrapping': 'off',
      'better-tailwindcss/enforce-canonical-classes': 'error',
    },
  },
];
