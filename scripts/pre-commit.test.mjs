import assert from 'node:assert/strict';
import test from 'node:test';

import { isMiseTomlFile, isRootPrettierFile } from './pre-commit-formatters.mjs';

test('classifies mise configuration files for the mise formatter', () => {
  // Given: repository, environment, module, and task configuration paths.
  const miseFiles = [
    'mise.toml',
    'mise.development.toml',
    'apps/api/mise.toml',
    'apps/web/mise.toml',
    'packages/node/configs/mise.toml',
    'mise/tasks/project.toml',
  ];

  // When/Then: every mise-owned TOML file selects the mise formatter.
  for (const file of miseFiles) {
    assert.equal(isMiseTomlFile(file), true);
  }
});

test('does not classify unrelated TOML files as mise configuration', () => {
  // Given: TOML files outside mise's configuration naming and task directory.
  const unrelatedFiles = ['Cargo.toml', 'infra/example.toml', 'mise/tasks/readme.md'];

  // When/Then: unrelated files remain outside the staged mise formatter.
  for (const file of unrelatedFiles) {
    assert.equal(isMiseTomlFile(file), false);
  }
});

test('routes supported root files and script modules to Prettier', () => {
  // Given / When / Then: root files and script modules use the root formatter.
  for (const file of [
    '.golangci.yaml',
    'commitlint.config.cjs',
    'package.json',
    'pnpm-workspace.yaml',
    'scripts/version.mjs',
  ]) {
    assert.equal(isRootPrettierFile(file), true, file);
  }
  for (const file of ['README.md', 'apps/web/package.json', 'scripts/fixture.json']) {
    assert.equal(isRootPrettierFile(file), false, file);
  }
});
