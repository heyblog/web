import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import {
  copyFileSync,
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  symlinkSync,
  writeFileSync,
} from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import test from 'node:test';

const repoRoot = resolve(import.meta.dirname, '..');

function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), 'heyblog-hook-security-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  for (const directory of ['.githooks', 'scripts', 'bin', 'apps/web', 'packages/node/configs']) {
    mkdirSync(join(root, directory), { recursive: true });
  }
  const env = { ...process.env, GIT_CONFIG_GLOBAL: '/dev/null', GIT_CONFIG_NOSYSTEM: '1' };
  const git = (...args) => {
    const result = spawnSync('git', args, { cwd: root, env, encoding: 'utf8' });
    assert.equal(result.status, 0, result.stderr);
  };
  const manifest = (file, description = '') => {
    writeFileSync(
      join(root, file),
      JSON.stringify({
        name: file.replaceAll('/', '-').replace('.json', ''),
        private: true,
        description,
      }),
    );
  };
  for (const file of [
    'package.json',
    'apps/web/package.json',
    'packages/node/configs/package.json',
  ]) {
    manifest(file);
  }
  writeFileSync(
    join(root, 'pnpm-workspace.yaml'),
    'packages:\n  - apps/web\n  - packages/node/*\n',
  );
  const install = spawnSync('pnpm', ['install', '--lockfile-only', '--ignore-scripts'], {
    cwd: root,
    env,
    encoding: 'utf8',
  });
  assert.equal(install.status, 0, install.stdout + install.stderr);
  git('init', '-q');
  git(
    'add',
    'package.json',
    'apps/web/package.json',
    'packages/node/configs/package.json',
    'pnpm-workspace.yaml',
    'pnpm-lock.yaml',
  );
  git(
    '-c',
    'user.name=Hook test',
    '-c',
    'user.email=hook@example.test',
    '-c',
    'commit.gpgsign=false',
    'commit',
    '-qm',
    'test: initialize fixture',
  );

  for (const file of ['pre-commit-security.mjs', 'pre-commit-formatters.mjs']) {
    copyFileSync(join(repoRoot, 'scripts', file), join(root, 'scripts', file));
  }
  copyFileSync(join(repoRoot, '.githooks/pre-commit'), join(root, '.githooks/pre-commit'));
  symlinkSync(join(repoRoot, 'node_modules'), join(root, 'node_modules'), 'dir');
  writeFileSync(
    join(root, 'bin/mise'),
    `#!/usr/bin/env node
const { appendFileSync } = require('node:fs');
const { spawnSync } = require('node:child_process');
const args = process.argv.slice(2);
appendFileSync('calls.jsonl', JSON.stringify(args) + '\\n');
if (args[0] === 'exec') {
  process.exit(spawnSync(args[2], args.slice(3), { stdio: 'inherit' }).status ?? 1);
}
if (args[1] === 'hooks:security') {
  process.exit(spawnSync(process.execPath, ['scripts/pre-commit-security.mjs'], { stdio: 'inherit' }).status ?? 1);
}
if (args[1] === 'security:dev') process.exit(Number(process.env.HOOK_TEST_DEV_STATUS ?? 0));
if (args[1] === 'security:node') process.exit(Number(process.env.HOOK_TEST_PROD_STATUS ?? 0));
`,
    { mode: 0o755 },
  );
  const run = (statuses = {}) =>
    spawnSync('sh', ['.githooks/pre-commit'], {
      cwd: root,
      env: { ...env, PATH: `${join(root, 'bin')}:${env.PATH}`, ...statuses },
      encoding: 'utf8',
    });
  const calls = () =>
    existsSync(join(root, 'calls.jsonl'))
      ? readFileSync(join(root, 'calls.jsonl'), 'utf8')
          .trim()
          .split('\n')
          .map((line) => JSON.parse(line))
      : [];
  return { root, git, manifest, run, calls };
}

test('ordinary code commits skip dependency checks', (t) => {
  // Given: a staged source file with unchanged dependency inputs.
  const f = fixture(t);
  writeFileSync(join(f.root, 'application.js'), 'export const value = 1;\n');
  f.git('add', 'application.js');
  // When: the actual pre-commit hook executes.
  const result = f.run();
  // Then: neither lockfile validation nor audits execute.
  assert.equal(result.status, 0, result.stderr);
  assert.ok(f.calls().every((args) => args[0] !== 'exec' && !args[1]?.startsWith('security:')));
});

for (const input of [
  'package.json',
  'apps/web/package.json',
  'packages/node/configs/package.json',
  'pnpm-workspace.yaml',
]) {
  test(`staged changes to ${input} trigger dependency checks`, (t) => {
    // Given: a staged dependency input compatible with the frozen lockfile.
    const f = fixture(t);
    if (input.endsWith('.json')) f.manifest(input, 'updated');
    else
      writeFileSync(
        join(f.root, input),
        'packages:\n  - apps/web\n  - packages/node/*\n# updated\n',
      );
    f.git('add', input);
    // When: the pre-commit hook executes.
    const result = f.run();
    // Then: lockfile validation and both audit tasks execute in order.
    assert.equal(result.status, 0, result.stdout + result.stderr);
    assert.deepEqual(
      f.calls().filter((args) => args[0] === 'exec' || args[1]?.startsWith('security:')),
      [
        [
          'exec',
          '--',
          'pnpm',
          'install',
          '--frozen-lockfile',
          '--lockfile-only',
          '--ignore-scripts',
        ],
        ['run', 'security:dev'],
        ['run', 'security:node'],
      ],
    );
  });
}

for (const change of ['delete', 'rename', 'delete-retained']) {
  test(`${change} of the lockfile cannot bypass dependency checks`, (t) => {
    // Given: a staged deletion, or a rename away from the canonical lockfile path.
    const f = fixture(t);
    const retained = change === 'delete-retained';
    if (retained) {
      f.git('rm', '--cached', 'pnpm-lock.yaml');
      writeFileSync(join(f.root, '.gitignore'), 'pnpm-lock.yaml\n');
      f.git('add', '.gitignore');
    } else if (change === 'delete') f.git('rm', 'pnpm-lock.yaml');
    else f.git('mv', 'pnpm-lock.yaml', 'previous-lock.yaml');
    // When: the pre-commit hook executes.
    const result = f.run();
    // Then: the removed lockfile is rejected, before validation if a worktree copy remains.
    assert.notEqual(result.status, 0);
    assert.equal(
      f.calls().some((args) => args[0] === 'exec'),
      !retained,
    );
    assert.ok(f.calls().every((args) => !args[1]?.startsWith('security:')));
  });
}

test('unstaged dependency inputs reject the commit before auditing', (t) => {
  // Given: a staged Web manifest and a different dependency input changed only in the worktree.
  const f = fixture(t);
  f.manifest('apps/web/package.json', 'staged');
  f.git('add', 'apps/web/package.json');
  f.manifest('package.json', 'unstaged');
  // When: the pre-commit hook executes.
  const result = f.run();
  // Then: it refuses to audit a different dependency snapshot.
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /package\.json/u);
  assert.ok(f.calls().every((args) => args[0] !== 'exec' && !args[1]?.startsWith('security:')));
});

test('outdated dependency lockfiles reject the commit before auditing', (t) => {
  // Given: a manifest adds a dependency without updating the lockfile.
  const f = fixture(t);
  writeFileSync(
    join(f.root, 'package.json'),
    JSON.stringify({ private: true, dependencies: { missing: '1.0.0' } }),
  );
  f.git('add', 'package.json');
  // When: the pre-commit hook executes with real frozen pnpm validation.
  const result = f.run();
  // Then: pnpm detects the mismatch and no audit runs.
  assert.notEqual(result.status, 0);
  assert.ok(f.calls().some((args) => args[0] === 'exec'));
  assert.ok(f.calls().every((args) => !args[1]?.startsWith('security:')));
});

test('development audit failure warns and permits the commit', (t) => {
  // Given: compatible staged inputs and a development audit failure.
  const f = fixture(t);
  f.manifest('package.json', 'updated');
  f.git('add', 'package.json');
  // When: the pre-commit hook executes.
  const result = f.run({ HOOK_TEST_DEV_STATUS: '1' });
  // Then: production is still audited and the hook succeeds with an actionable warning.
  assert.equal(result.status, 0, result.stdout + result.stderr);
  assert.match(result.stderr, /Warning:/u);
  assert.match(result.stderr, /mise run security:dev/u);
  assert.ok(f.calls().some((args) => args[1] === 'security:node'));
});

test('production audit failure blocks the commit', (t) => {
  // Given: compatible staged inputs and a production audit failure.
  const f = fixture(t);
  f.manifest('package.json', 'updated');
  f.git('add', 'package.json');
  // When: the pre-commit hook executes.
  const result = f.run({ HOOK_TEST_PROD_STATUS: '1' });
  // Then: the failing production audit status becomes the hook status.
  assert.equal(result.status, 1);
  assert.ok(f.calls().some((args) => args[1] === 'security:node'));
});
