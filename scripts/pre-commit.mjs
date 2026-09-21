import { execFileSync } from 'node:child_process';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import {
  formatGoFiles,
  formatMiseFiles,
  formatNodeModuleFiles,
  formatRootFiles,
  goModuleConfigs,
  isMiseTomlFile,
  isRootPrettierFile,
  nodeModuleConfigs,
  runCommand,
} from './pre-commit-formatters.mjs';

const repoRoot = resolve(import.meta.dirname, '..');
const trackedRoots = [
  ...goModuleConfigs.map((config) => `${config.dir}/`),
  ...nodeModuleConfigs.map((config) => `${config.dir}/`),
];

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
  return (
    isRootPrettierFile(file) ||
    isMiseTomlFile(file) ||
    trackedRoots.some((root) => file.startsWith(root))
  );
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

function getModuleFiles(files, moduleDir) {
  const modulePrefix = `${moduleDir}/`;
  return files.filter((file) => file.startsWith(modulePrefix));
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

  formattedFiles.push(...formatMiseFiles(trackedFiles));
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
