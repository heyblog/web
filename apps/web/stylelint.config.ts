import postcssHtml from 'postcss-html';

const stylelintConfig = {
  extends: ['stylelint-config-standard'],
  ignoreFiles: ['**/node_modules/**', '**/dist/**', '**/coverage/**', '**/.astro/**'],
  overrides: [
    {
      files: ['**/*.{astro,svelte}'],
      customSyntax: postcssHtml,
    },
  ],
  rules: {
    'at-rule-no-unknown': [
      true,
      {
        ignoreAtRules: [
          'apply',
          'config',
          'custom-variant',
          'plugin',
          'source',
          'tailwind',
          'theme',
          'utility',
          'variant',
        ],
      },
    ],
    'custom-property-empty-line-before': null,
    'custom-property-pattern': null,
    'declaration-empty-line-before': null,
    'import-notation': null,
    'media-feature-range-notation': null,
    'value-keyword-case': null,
  },
};

export default stylelintConfig;
