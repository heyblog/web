const sharedPrettierConfig = {
  printWidth: 100,
  singleQuote: true,
  trailingComma: 'all',
  semi: true,
  arrowParens: 'always',
  plugins: ['prettier-plugin-astro', 'prettier-plugin-svelte', 'prettier-plugin-tailwindcss'],
};

export default sharedPrettierConfig;
