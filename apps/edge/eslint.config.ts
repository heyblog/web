import { createModuleEslintConfig } from '../../packages/node/configs/shared/eslint.ts';
export default [
  { ignores: ['dist/**'] },
  ...createModuleEslintConfig({ moduleDir: import.meta.dirname, typeAware: true }),
];
