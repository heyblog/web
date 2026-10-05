import { execFileSync } from 'node:child_process';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import crossSpawn from 'cross-spawn';

import { nodeModuleConfigs } from './pre-commit-formatters.mjs';

const repoRoot = resolve(import.meta.dirname, '..');
const dependencyInputs = new Set([
  'package.json',
  'pnpm-workspace.yaml',
  'pnpm-lock.yaml',
  ...nodeModuleConfigs.map((config) => `${config.dir}/package.json`),
]);

function readChangedFiles(args) {
  const [command, ...options] = args;
  return execFileSync('git', [command, '-z', ...options], { cwd: repoRoot })
    .toString('utf8')
    .split('\0')
    .filter((file) => dependencyInputs.has(file));
}

function runMise(args) {
  const result = crossSpawn.sync('mise', args, { cwd: repoRoot, stdio: 'inherit' });
  if (result.error) {
    console.error('Could not start mise for dependency checks. Ensure mise is installed.');
  }
  return result.status ?? 1;
}

export function main() {
  const stagedInputs = readChangedFiles([
    'diff',
    '--cached',
    '--name-only',
    '--no-renames',
    '--diff-filter=ACMD',
  ]);
  if (stagedInputs.length === 0) return 0;

  const unstagedInputs = [
    ...readChangedFiles(['diff', '--name-only', '--no-renames']),
    ...readChangedFiles(['ls-files', '--others', '--', ...dependencyInputs]),
  ];
  if (unstagedInputs.length > 0) {
    console.error('Dependency checks aborted: dependency inputs contain unstaged changes.');
    console.error('Fully stage or stash these files before committing:');
    for (const file of unstagedInputs) console.error(`- ${file}`);
    return 1;
  }

  const lockfileStatus = runMise([
    'exec',
    '--',
    'pnpm',
    'install',
    '--frozen-lockfile',
    '--lockfile-only',
    '--ignore-scripts',
  ]);
  if (lockfileStatus !== 0) return lockfileStatus;

  const devStatus = runMise(['run', 'security:dev']);
  if (devStatus !== 0) {
    console.warn('Warning: development dependency audit failed; this does not block the commit.');
    console.warn('Review the audit output above. Re-run with: mise run security:dev');
  }

  return runMise(['run', 'security:node']);
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  process.exitCode = main();
}
