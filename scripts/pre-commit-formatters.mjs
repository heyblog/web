import { readFileSync, writeFileSync } from 'node:fs';
import { relative, resolve } from 'node:path';

import crossSpawn from 'cross-spawn';

const repoRoot = resolve(import.meta.dirname, '..');
const rootPrettierConfig = './packages/node/configs/prettier.config.ts';
const rootPrettierFileNames = new Set([
  '.golangci.yaml',
  'commitlint.config.cjs',
  'package.json',
  'pnpm-workspace.yaml',
]);
const prettierExtensions = new Set([
  '.astro',
  '.cjs',
  '.css',
  '.js',
  '.json',
  '.mjs',
  '.svelte',
  '.ts',
]);
const eslintExtensions = new Set(['.astro', '.cjs', '.js', '.mjs', '.svelte', '.ts']);
const stylelintExtensions = new Set(['.astro', '.css', '.svelte']);

export const goModuleConfigs = [
  {
    dir: 'apps/api',
    config: '../../.golangci.yaml',
  },
];

export const nodeModuleConfigs = [
  {
    dir: 'apps/web',
    prettierConfig: './prettier.config.ts',
    eslintConfig: './eslint.config.ts',
    stylelintConfig: './stylelint.config.ts',
  },
  {
    dir: 'packages/node/configs',
    prettierConfig: './prettier.config.ts',
    eslintConfig: './eslint.config.ts',
  },
];

function getExtension(filePath) {
  const lastDotIndex = filePath.lastIndexOf('.');
  return lastDotIndex === -1 ? '' : filePath.slice(lastDotIndex);
}

export function runCommand(command, args, cwd = repoRoot, options = {}) {
  const { env = process.env, runner = crossSpawn } = options;
  const result = runner.sync(command, args, {
    cwd,
    env,
    stdio: 'inherit',
  });

  if (result.error) {
    throw result.error;
  }

  if (result.status !== 0) {
    process.exit(result.status ?? 1);
  }
}

export function isMiseTomlFile(file) {
  return (
    /(^|\/)mise(?:\.[^/]+)?\.toml$/.test(file) ||
    (file.startsWith('mise/tasks/') && file.endsWith('.toml'))
  );
}

export function isRootPrettierFile(file) {
  if (rootPrettierFileNames.has(file)) {
    return true;
  }

  return file.startsWith('scripts/') && getExtension(file) === '.mjs';
}

export function formatMiseFiles(files, root = repoRoot, runner = crossSpawn) {
  const miseFiles = files.filter(isMiseTomlFile);

  for (const file of miseFiles) {
    const path = resolve(root, file);
    const result = runner.sync('mise', ['fmt', '--stdin'], {
      cwd: root,
      encoding: 'utf8',
      input: readFileSync(path, 'utf8'),
      stdio: ['pipe', 'pipe', 'inherit'],
    });

    if (result.error) {
      throw result.error;
    }

    if (result.status !== 0) {
      process.exit(result.status ?? 1);
    }

    writeFileSync(path, result.stdout, 'utf8');
  }

  return miseFiles;
}

export function formatRootFiles(files) {
  const prettierFiles = files.filter(isRootPrettierFile);

  if (prettierFiles.length === 0) {
    return [];
  }

  runCommand('pnpm', [
    'exec',
    'prettier',
    '--config',
    rootPrettierConfig,
    '--write',
    ...prettierFiles,
  ]);
  return prettierFiles;
}

export function formatGoFiles(files, run = runCommand, root = repoRoot) {
  const goFiles = files.filter((file) => file.endsWith('.go'));

  if (goFiles.length === 0) {
    return [];
  }

  run('go', ['tool', 'goimports', '-w', ...goFiles], root);
  return goFiles;
}

export function formatNodeModuleFiles(files, moduleConfig) {
  const prettierFiles = files.filter((file) => {
    return (
      prettierExtensions.has(getExtension(file)) ||
      file.endsWith('/package.json') ||
      file === 'package.json'
    );
  });
  const eslintFiles = files.filter((file) => eslintExtensions.has(getExtension(file)));
  const stylelintFiles = moduleConfig.stylelintConfig
    ? files.filter((file) => stylelintExtensions.has(getExtension(file)))
    : [];
  const moduleDir = resolve(repoRoot, moduleConfig.dir);

  if (eslintFiles.length > 0) {
    runCommand(
      'pnpm',
      [
        'exec',
        'eslint',
        '--config',
        moduleConfig.eslintConfig,
        '--fix',
        ...eslintFiles.map((file) => relative(moduleConfig.dir, file)),
      ],
      moduleDir,
    );
  }

  if (stylelintFiles.length > 0 && moduleConfig.stylelintConfig) {
    runCommand(
      'pnpm',
      [
        'exec',
        'stylelint',
        '--config',
        moduleConfig.stylelintConfig,
        '--fix',
        ...stylelintFiles.map((file) => relative(moduleConfig.dir, file)),
      ],
      moduleDir,
    );
  }

  if (prettierFiles.length > 0) {
    runCommand(
      'pnpm',
      [
        'exec',
        'prettier',
        '--config',
        moduleConfig.prettierConfig,
        '--write',
        ...prettierFiles.map((file) => relative(moduleConfig.dir, file)),
      ],
      moduleDir,
    );
  }

  return [...new Set([...prettierFiles, ...eslintFiles, ...stylelintFiles])];
}
