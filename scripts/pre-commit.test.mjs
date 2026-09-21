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

test('root prettier classification no longer includes Task configuration', () => {
  // Given: removed Task paths and supported root source files.

  // When/Then: Task YAML is excluded while root JavaScript remains supported.
  assert.equal(isRootPrettierFile('Taskfile.yaml'), false);
  assert.equal(isRootPrettierFile('taskfiles/project.yaml'), false);
  assert.equal(isRootPrettierFile('scripts/version.mjs'), true);
});
