import sharedPrettierConfig from '@heyblog/configs/shared/prettier';

export default {
  ...sharedPrettierConfig,
  plugins: ['prettier-plugin-astro', 'prettier-plugin-svelte'],
};
