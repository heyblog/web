import { execFileSync } from 'node:child_process';
import { relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import crossSpawn from 'cross-spawn';

const repoRoot = resolve(import.meta.dirname, '..');

const rootPrettierConfig = {
  prettierConfig: './packages/node/configs/prettier.config.ts',
};

const goModuleConfigs = [
  {
    dir: 'apps/api',
    config: '../../.golangci.yaml',
  },
];

const nodeModuleConfigs = [
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

const trackedRoots = [
  ...goModuleConfigs.map((config) => `${config.dir}/`),
  ...nodeModuleConfigs.map((config) => `${config.dir}/`),
];

const rootPrettierFileNames = new Set([
  '.golangci.yaml',
  'Taskfile.yaml',
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

export function parseGitFileList(output) {
  return output
    .toString('utf8')
    .split('\0')
    .filter((file) => file.length > 0);
}

function readGitFileList(args) {
  const output = execFileSync('git', [...args, '-z'], {
    cwd: repoRoot,
  });

  return parseGitFileList(output);
}

function parseCliArgs() {
  const normalizedArgs = process.argv.slice(2).filter((arg) => arg !== '--');
  const noStage = normalizedArgs.includes('--no-stage');
  const files = normalizedArgs.filter((arg) => arg !== '--no-stage');

  return {
    files,
    noStage,
  };
}

function isTrackedFile(file) {
  return isRootPrettierFile(file) || trackedRoots.some((root) => file.startsWith(root));
}

export function findPartiallyStagedFiles(files, unstagedFiles) {
  const unstagedSet = new Set(unstagedFiles);
  return files.filter((file) => unstagedSet.has(file));
}

function ensureNoPartiallyStagedFiles(files) {
  const partiallyStagedFiles = findPartiallyStagedFiles(
    files,
    readGitFileList(['diff', '--name-only']),
  );

  if (partiallyStagedFiles.length === 0) {
    return;
  }

  console.error('Pre-commit aborted: partially staged files are not supported.');
  console.error('Please fully stage or stash the following files first:');

  for (const file of partiallyStagedFiles) {
    console.error(`- ${file}`);
  }

  process.exit(1);
}

function stageFiles(files) {
  if (files.length === 0) {
    return;
  }

  runCommand('git', ['add', '--', ...files]);
}

function getExtension(filePath) {
  const lastDotIndex = filePath.lastIndexOf('.');
  return lastDotIndex === -1 ? '' : filePath.slice(lastDotIndex);
}

function getModuleFiles(files, moduleDir) {
  const modulePrefix = `${moduleDir}/`;
  return files.filter((file) => file.startsWith(modulePrefix));
}

export function isRootPrettierFile(file) {
  if (rootPrettierFileNames.has(file)) {
    return true;
  }

  if (/(^|\/)Taskfile\.yaml$/.test(file)) {
    return true;
  }

  if (file.startsWith('scripts/')) {
    return getExtension(file) === '.mjs';
  }

  if (file.startsWith('taskfiles/')) {
    return getExtension(file) === '.yaml';
  }

  return false;
}

function formatRootFiles(files) {
  const prettierFiles = files.filter(isRootPrettierFile);

  if (prettierFiles.length === 0) {
    return [];
  }

  runCommand(
    'pnpm',
    [
      'exec',
      'prettier',
      '--config',
      rootPrettierConfig.prettierConfig,
      '--write',
      ...prettierFiles,
    ],
    repoRoot,
  );

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

function formatNodeModuleFiles(files, moduleConfig) {
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

export function main() {
  const { files: cliFiles, noStage } = parseCliArgs();
  const files =
    cliFiles.length > 0
      ? cliFiles
      : readGitFileList(['diff', '--cached', '--name-only', '--diff-filter=ACMR']);

  if (files.length === 0) {
    process.exit(0);
  }

  const trackedFiles = files.filter(isTrackedFile);

  if (trackedFiles.length === 0) {
    process.exit(0);
  }

  if (!noStage) {
    ensureNoPartiallyStagedFiles(trackedFiles);
  }

  const formattedFiles = [];

  formattedFiles.push(...formatRootFiles(trackedFiles));

  for (const moduleConfig of goModuleConfigs) {
    const moduleFiles = getModuleFiles(trackedFiles, moduleConfig.dir);
    formattedFiles.push(...formatGoFiles(moduleFiles));
  }

  for (const moduleConfig of nodeModuleConfigs) {
    const moduleFiles = getModuleFiles(trackedFiles, moduleConfig.dir);
    formattedFiles.push(...formatNodeModuleFiles(moduleFiles, moduleConfig));
  }

  if (!noStage) {
    stageFiles([...new Set(formattedFiles)]);
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main();
}
