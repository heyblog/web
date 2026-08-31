import { createModuleEslintConfig } from './packages/node/configs/shared/eslint.ts';

export default createModuleEslintConfig({
  moduleDir: import.meta.dirname,
  typeAware: true,
});
