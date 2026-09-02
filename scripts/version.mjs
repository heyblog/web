import { readdir, readFile, writeFile } from 'node:fs/promises';
import { join, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const repositoryRoot = resolve(import.meta.dirname, '..');
const versionPattern = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/u;
const bumpKinds = new Set(['major', 'minor', 'patch']);
const ignoredDirectories = new Set([
  '.astro',
  '.cache',
  '.git',
  'contents',
  'coverage',
  'dist',
  'node_modules',
  'tmp',
]);
const usage = 'Usage: node scripts/version.mjs <show|check|set X.Y.Z|bump patch|minor|major>';

export function parseVersion(value) {
  const match = versionPattern.exec(value);

  if (!match) {
    throw new Error(`Invalid project version "${value}"; expected X.Y.Z.`);
  }

  const version = {
    major: Number(match[1]),
    minor: Number(match[2]),
    patch: Number(match[3]),
  };

  if (Object.values(version).some((part) => !Number.isSafeInteger(part))) {
    throw new Error(`Invalid project version "${value}"; expected safe X.Y.Z integers.`);
  }

  return version;
}

export function bumpVersion(value, kind) {
  if (!bumpKinds.has(kind)) {
    throw new Error(`Invalid bump kind "${kind}"; expected patch|minor|major.`);
  }

  const version = parseVersion(value);
  const part = version[kind];

  if (part === Number.MAX_SAFE_INTEGER) {
    throw new Error(`Cannot bump ${kind}; the version component is already at its safe limit.`);
  }

  if (kind === 'major') {
    return `${version.major + 1}.0.0`;
  }

  if (kind === 'minor') {
    return `${version.major}.${version.minor + 1}.0`;
  }

  return `${version.major}.${version.minor}.${version.patch + 1}`;
}

export function parseCommand(args) {
  const [command, argument, ...extraArguments] = args;

  if (command === 'show' || command === 'check') {
    if (argument !== undefined || extraArguments.length > 0) {
      throw new Error(usage);
    }

    return { command };
  }

  if (command === 'set') {
    if (argument === undefined || extraArguments.length > 0) {
      throw new Error(usage);
    }

    parseVersion(argument);
    return { command, version: argument };
  }

  if (command === 'bump') {
    if (argument === undefined || extraArguments.length > 0 || !bumpKinds.has(argument)) {
      throw new Error(usage);
    }

    return { command, kind: argument };
  }

  throw new Error(usage);
}

async function findPackageManifests(directory, root, manifests) {
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const path = join(directory, entry.name);

    if (entry.isDirectory() && !ignoredDirectories.has(entry.name)) {
      await findPackageManifests(path, root, manifests);
      continue;
    }

    if (entry.isFile() && entry.name === 'package.json') {
      manifests.push(relative(root, path));
    }
  }
}

async function listProjectPackageManifests(root) {
  const manifests = ['package.json'];

  for (const directory of ['apps', 'packages']) {
    await findPackageManifests(join(root, directory), root, manifests);
  }

  return manifests.sort();
}

async function readCanonicalVersion(root) {
  const path = join(root, 'VERSION');
  let contents;

  try {
    contents = await readFile(path, 'utf8');
  } catch (error) {
    if (error?.code === 'ENOENT') {
      throw new Error('VERSION is missing from the repository root.', { cause: error });
    }

    throw error;
  }

  const version = contents.endsWith('\n') ? contents.slice(0, -1) : contents;

  if (contents !== `${version}\n`) {
    throw new Error('VERSION must contain one X.Y.Z value followed by a newline.');
  }

  parseVersion(version);
  return version;
}

export async function checkRepositoryVersion(root = repositoryRoot) {
  const version = await readCanonicalVersion(root);
  const manifests = await listProjectPackageManifests(root);
  const violations = [];

  for (const manifest of manifests) {
    const path = join(root, manifest);
    let parsedManifest;

    try {
      parsedManifest = JSON.parse(await readFile(path, 'utf8'));
    } catch (error) {
      throw new Error(`Cannot read ${manifest}: ${error.message}`, { cause: error });
    }

    if (Object.hasOwn(parsedManifest, 'version')) {
      violations.push(`${manifest} declares a version field`);
    }
  }

  if (violations.length > 0) {
    throw new Error(
      `Project package manifests must inherit VERSION:\n- ${violations.join('\n- ')}`,
    );
  }

  return { manifests, version };
}

export async function setRepositoryVersion(root, version) {
  parseVersion(version);
  const current = await checkRepositoryVersion(root);
  await writeFile(join(root, 'VERSION'), `${version}\n`);

  return { previousVersion: current.version, version };
}

export async function runCommand(args, root = repositoryRoot, output = console.log) {
  const command = parseCommand(args);

  if (command.command === 'show') {
    const result = await checkRepositoryVersion(root);
    output(result.version);
    return;
  }

  if (command.command === 'check') {
    const result = await checkRepositoryVersion(root);
    output(`Project version ${result.version} is valid.`);
    return;
  }

  const current = await checkRepositoryVersion(root);
  const version =
    command.command === 'set' ? command.version : bumpVersion(current.version, command.kind);
  const result = await setRepositoryVersion(root, version);
  output(`Project version: ${result.previousVersion} -> ${result.version}`);
}

async function main() {
  try {
    await runCommand(process.argv.slice(2));
  } catch (error) {
    console.error(`Version command failed: ${error.message}`);
    process.exitCode = 1;
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  await main();
}
