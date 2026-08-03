import { randomUUID } from 'node:crypto';
import { lstat, mkdir, rename, rm, writeFile } from 'node:fs/promises';
import { basename, dirname, join, resolve, sep } from 'node:path';

import { contentSources } from '../apps/web/src/content-sources.mjs';

const repositoryOwner = 'heyblog';
const repositoryName = '.github';
const repositoryUrl = `https://github.com/${repositoryOwner}/${repositoryName}.git`;
const repositoryBranch = 'main';
const contentPrefix = 'contents/';
const repositoryRoot = resolve(import.meta.dirname, '..');
const webDirectory = join(repositoryRoot, 'apps', 'web');
const targetDirectory = join(webDirectory, 'contents');
const syncDirectory = join(webDirectory, `.content-sync-${randomUUID()}`);
const stagedDirectory = join(syncDirectory, 'contents');
const backupDirectory = join(syncDirectory, 'previous-contents');
let preserveSyncDirectory = false;

const allowedFileModes = new Set(['100644', '100755']);
const windowsReservedNames = new Set([
  'AUX',
  'CON',
  'NUL',
  'PRN',
  ...Array.from({ length: 9 }, (_, index) => `COM${index + 1}`),
  ...Array.from({ length: 9 }, (_, index) => `LPT${index + 1}`),
]);

function containsControlCharacter(value) {
  return Array.from(value).some((character) => {
    const codePoint = character.codePointAt(0);
    return codePoint !== undefined && (codePoint <= 31 || codePoint === 127);
  });
}

function assertGitSha(value, label) {
  if (typeof value !== 'string' || !/^[0-9a-f]{40}$/u.test(value)) {
    throw new Error(`${label} did not contain a valid commit SHA.`);
  }

  return value;
}

async function fetchJson(url, label) {
  const response = await fetch(url, {
    headers: {
      Accept: 'application/vnd.github+json',
      'User-Agent': 'heyblog-content-sync',
    },
  });

  if (!response.ok) {
    throw new Error(`${label} failed with HTTP ${response.status}.`);
  }

  return response.json();
}

function encodeRepositoryPath(filePath) {
  return filePath
    .split('/')
    .map((segment) => encodeURIComponent(segment))
    .join('/');
}

function assertPortableSegment(segment, repositoryPath) {
  const reservedName = segment.split('.')[0]?.toUpperCase();

  if (
    segment.length === 0 ||
    segment === '.' ||
    segment === '..' ||
    /[<>:"\\|?*]/u.test(segment) ||
    containsControlCharacter(segment) ||
    /[ .]$/u.test(segment) ||
    (reservedName && windowsReservedNames.has(reservedName))
  ) {
    throw new Error(`Content path is not portable: ${repositoryPath}.`);
  }
}

function resolveContentPath(repositoryPath) {
  const relativePath = repositoryPath.slice(contentPrefix.length);
  const segments = relativePath.split('/');

  for (const segment of segments) {
    assertPortableSegment(segment, repositoryPath);
  }

  const destination = resolve(stagedDirectory, ...segments);
  const expectedPrefix = `${resolve(stagedDirectory)}${sep}`;

  if (!destination.startsWith(expectedPrefix)) {
    throw new Error(`Content path escapes the staging directory: ${repositoryPath}.`);
  }

  return {
    destination,
    relativePath,
  };
}

function selectContentFiles(tree) {
  if (!Array.isArray(tree)) {
    throw new Error('GitHub tree response did not contain an entry list.');
  }

  const portablePaths = new Set();
  const selectedFiles = [];

  for (const entry of tree) {
    if (
      !entry ||
      typeof entry.path !== 'string' ||
      !entry.path.startsWith(contentPrefix) ||
      entry.type === 'tree'
    ) {
      continue;
    }

    if (entry.type !== 'blob' || !allowedFileModes.has(entry.mode)) {
      throw new Error(`Unsupported content entry: ${entry.path}.`);
    }

    const resolvedPath = resolveContentPath(entry.path);
    const portablePath = resolvedPath.relativePath.normalize('NFC').toLocaleLowerCase('en-US');

    if (portablePaths.has(portablePath)) {
      throw new Error(`Content paths collide on case-insensitive filesystems: ${entry.path}.`);
    }

    portablePaths.add(portablePath);
    selectedFiles.push({
      ...resolvedPath,
      repositoryPath: entry.path,
      size: entry.size,
    });
  }

  return selectedFiles;
}

function validateConfiguredSources(files) {
  const failures = [];

  for (const [collection, source] of Object.entries(contentSources)) {
    if (source.kind === 'file') {
      const file = files.find((candidate) => candidate.relativePath === source.path);

      if (!file || file.size === 0) {
        failures.push(`${collection}: ${contentPrefix}${source.path}`);
      }

      continue;
    }

    const directoryPrefix = `${source.path}/`;
    const hasMatchingFile = files.some((file) => {
      return (
        file.relativePath.startsWith(directoryPrefix) &&
        source.extensions.some((extension) => file.relativePath.endsWith(extension))
      );
    });

    if (!hasMatchingFile) {
      failures.push(`${collection}: ${contentPrefix}${source.path}/`);
    }
  }

  if (failures.length > 0) {
    throw new Error(`Configured content sources are empty:\n- ${failures.join('\n- ')}`);
  }
}

async function runWithConcurrency(items, concurrency, operation) {
  let nextIndex = 0;

  async function worker() {
    while (nextIndex < items.length) {
      const item = items[nextIndex];
      nextIndex += 1;
      await operation(item);
    }
  }

  await Promise.all(Array.from({ length: Math.min(concurrency, items.length) }, () => worker()));
}

async function downloadContentFile(contentSha, file) {
  const encodedPath = encodeRepositoryPath(file.repositoryPath);
  const url = `https://raw.githubusercontent.com/${repositoryOwner}/${repositoryName}/${contentSha}/${encodedPath}`;
  const response = await fetch(url, {
    headers: {
      'User-Agent': 'heyblog-content-sync',
    },
  });

  if (!response.ok) {
    throw new Error(`Downloading ${file.repositoryPath} failed with HTTP ${response.status}.`);
  }

  await mkdir(dirname(file.destination), { recursive: true });
  await writeFile(file.destination, Buffer.from(await response.arrayBuffer()));
}

async function pathExists(filePath) {
  try {
    await lstat(filePath);
    return true;
  } catch (error) {
    if (error?.code === 'ENOENT') {
      return false;
    }

    throw error;
  }
}

async function replaceContents() {
  const hadPreviousContents = await pathExists(targetDirectory);

  if (hadPreviousContents) {
    await rename(targetDirectory, backupDirectory);
  }

  try {
    await rename(stagedDirectory, targetDirectory);
  } catch (replacementError) {
    if (!hadPreviousContents) {
      throw replacementError;
    }

    try {
      await rename(backupDirectory, targetDirectory);
    } catch (rollbackError) {
      preserveSyncDirectory = true;
      throw new AggregateError(
        [replacementError, rollbackError],
        `Content replacement and rollback both failed; recovery data remains in ${syncDirectory}.`,
        { cause: rollbackError },
      );
    }

    throw replacementError;
  }
}

async function resolveRemoteSnapshot() {
  const encodedBranch = encodeURIComponent(repositoryBranch);
  const commitUrl = `https://api.github.com/repos/${repositoryOwner}/${repositoryName}/commits/${encodedBranch}`;
  const commitResponse = await fetchJson(commitUrl, `Resolving content branch ${repositoryBranch}`);
  const contentSha = assertGitSha(commitResponse?.sha, 'GitHub commit response');
  const treeSha = assertGitSha(commitResponse?.commit?.tree?.sha, 'GitHub commit tree response');
  const treeUrl = `https://api.github.com/repos/${repositoryOwner}/${repositoryName}/git/trees/${treeSha}?recursive=1`;
  const treeResponse = await fetchJson(treeUrl, `Loading content tree ${treeSha}`);

  if (treeResponse?.truncated === true) {
    throw new Error('GitHub truncated the content tree response.');
  }

  return {
    contentSha,
    files: selectContentFiles(treeResponse?.tree),
  };
}

async function main() {
  await mkdir(stagedDirectory, { recursive: true });

  try {
    const snapshot = await resolveRemoteSnapshot();

    if (snapshot.files.length === 0) {
      throw new Error(`No files found below ${contentPrefix}.`);
    }

    validateConfiguredSources(snapshot.files);
    await runWithConcurrency(snapshot.files, 8, (file) =>
      downloadContentFile(snapshot.contentSha, file),
    );
    await writeFile(
      join(stagedDirectory, '.source-revision'),
      [
        `repository=${repositoryUrl}`,
        `branch=${repositoryBranch}`,
        `commit=${snapshot.contentSha}`,
        '',
      ].join('\n'),
      'utf8',
    );
    await replaceContents();
    console.log(
      `Synced ${repositoryOwner}/${repositoryName}:${repositoryBranch} at ${snapshot.contentSha}.`,
    );
  } finally {
    if (!preserveSyncDirectory) {
      try {
        await rm(syncDirectory, { force: true, recursive: true });
      } catch (error) {
        console.warn(`Unable to remove temporary content directory ${basename(syncDirectory)}.`);
        console.warn(error);
      }
    }
  }
}

main().catch((error) => {
  console.error(error instanceof Error ? error.message : error);
  process.exitCode = 1;
});
