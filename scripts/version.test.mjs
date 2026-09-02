import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdir, mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

import {
  bumpVersion,
  checkRepositoryVersion,
  parseCommand,
  parseVersion,
  setRepositoryVersion,
} from './version.mjs';

async function createRepositoryFixture({ packageVersion } = {}) {
  const repositoryRoot = await mkdtemp(join(tmpdir(), 'heyblog-version-'));

  await mkdir(join(repositoryRoot, 'apps', 'web'), { recursive: true });
  await mkdir(join(repositoryRoot, 'packages', 'node', 'configs'), { recursive: true });
  await writeFile(join(repositoryRoot, 'VERSION'), '0.1.5\n');
  await writeFile(join(repositoryRoot, 'package.json'), '{"name":"heyblog-repo","private":true}\n');
  await writeFile(
    join(repositoryRoot, 'apps', 'web', 'package.json'),
    `${JSON.stringify({ name: 'heyblog-web', ...(packageVersion && { version: packageVersion }) })}\n`,
  );
  await writeFile(
    join(repositoryRoot, 'packages', 'node', 'configs', 'package.json'),
    '{"name":"@heyblog/configs","private":true}\n',
  );

  return repositoryRoot;
}

test('parseVersion accepts only strict safe X.Y.Z versions', () => {
  assert.deepEqual(parseVersion('0.1.5'), { major: 0, minor: 1, patch: 5 });

  for (const invalidVersion of [
    'v0.1.5',
    '0.1',
    '0.1.5-beta.1',
    '00.1.5',
    '0.01.5',
    '0.1.05',
    '9007199254740992.0.0',
  ]) {
    assert.throws(() => parseVersion(invalidVersion), /X\.Y\.Z/);
  }
});

test('bumpVersion applies standard semantic version increments', () => {
  assert.equal(bumpVersion('0.1.5', 'patch'), '0.1.6');
  assert.equal(bumpVersion('0.1.5', 'minor'), '0.2.0');
  assert.equal(bumpVersion('0.1.5', 'major'), '1.0.0');
});

test('bumpVersion rejects unsafe integer overflow', () => {
  assert.throws(() => bumpVersion('0.0.9007199254740991', 'patch'), /Cannot bump patch/);
  assert.throws(() => bumpVersion('0.9007199254740991.0', 'minor'), /Cannot bump minor/);
  assert.throws(() => bumpVersion('9007199254740991.0.0', 'major'), /Cannot bump major/);
});

test('parseCommand rejects missing and unsupported arguments', () => {
  assert.throws(() => parseCommand([]), /Usage/);
  assert.throws(() => parseCommand(['set']), /Usage/);
  assert.throws(() => parseCommand(['bump', 'build']), /patch\|minor\|major/);
  assert.throws(() => parseCommand(['show', 'extra']), /Usage/);
});

test('repository version check accepts the canonical file and versionless manifests', async (t) => {
  const repositoryRoot = await createRepositoryFixture();
  t.after(() => rm(repositoryRoot, { force: true, recursive: true }));

  const result = await checkRepositoryVersion(repositoryRoot);

  assert.equal(result.version, '0.1.5');
  assert.deepEqual(result.manifests, [
    'apps/web/package.json',
    'package.json',
    'packages/node/configs/package.json',
  ]);
});

test('repository version check rejects package-level version fields', async (t) => {
  const repositoryRoot = await createRepositoryFixture({ packageVersion: '0.1.5' });
  t.after(() => rm(repositoryRoot, { force: true, recursive: true }));

  await assert.rejects(
    checkRepositoryVersion(repositoryRoot),
    /apps\/web\/package\.json declares a version field/,
  );
});

test('repository version check rejects a missing VERSION file', async (t) => {
  const repositoryRoot = await createRepositoryFixture();
  t.after(() => rm(repositoryRoot, { force: true, recursive: true }));
  await rm(join(repositoryRoot, 'VERSION'));

  await assert.rejects(checkRepositoryVersion(repositoryRoot), /VERSION is missing/);
});

test('repository version check rejects malformed VERSION contents', async (t) => {
  const repositoryRoot = await createRepositoryFixture();
  t.after(() => rm(repositoryRoot, { force: true, recursive: true }));
  const versionPath = join(repositoryRoot, 'VERSION');

  for (const [contents, expectedError] of [
    ['0.1.5', /followed by a newline/],
    ['0.1.5\nextra\n', /expected X\.Y\.Z/],
    ['v0.1.5\n', /expected X\.Y\.Z/],
  ]) {
    await writeFile(versionPath, contents);
    await assert.rejects(checkRepositoryVersion(repositoryRoot), expectedError);
  }
});

test('setRepositoryVersion changes only the canonical VERSION file', async (t) => {
  const repositoryRoot = await createRepositoryFixture();
  t.after(() => rm(repositoryRoot, { force: true, recursive: true }));
  const manifestPath = join(repositoryRoot, 'apps', 'web', 'package.json');
  const manifestBefore = await readFile(manifestPath, 'utf8');

  const result = await setRepositoryVersion(repositoryRoot, '2.3.4');

  assert.deepEqual(result, { previousVersion: '0.1.5', version: '2.3.4' });
  assert.equal(await readFile(join(repositoryRoot, 'VERSION'), 'utf8'), '2.3.4\n');
  assert.equal(await readFile(manifestPath, 'utf8'), manifestBefore);
});

test('version CLI reports invalid usage without exposing a stack trace', () => {
  const scriptPath = fileURLToPath(new URL('./version.mjs', import.meta.url));
  const result = spawnSync(process.execPath, [scriptPath, 'set'], { encoding: 'utf8' });

  assert.equal(result.status, 1);
  assert.match(result.stderr, /^Version command failed: Usage:/);
  assert.doesNotMatch(result.stderr, /\n\s+at /);
});
