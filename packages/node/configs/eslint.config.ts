import { createModuleEslintConfig } from './shared/eslint.ts';

export default createModuleEslintConfig({
  moduleDir: import.meta.dirname,
  typeAware: true,
});
