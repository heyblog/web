import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import type { AstroIntegration } from 'astro';

export type BuildMetadata = {
  readonly component: string;
  readonly version: string;
  readonly ref: string;
  readonly commit: string;
  readonly shortCommit: string;
  readonly commitTime: string;
  readonly commitUrl: string;
  readonly buildTime: string;
};

type BuildMetadataOptions = {
  readonly repositoryRoot: string;
  readonly environment: Readonly<Record<string, string | undefined>>;
  readonly now: Date;
  readonly runGit: (args: readonly string[]) => string;
  readonly warn?: (message: string) => void;
};

const component = 'heyblog-web';
const versionPattern = /^(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)$/;
const commitPattern = /^[0-9a-f]{7,64}$/i;
const githubSegmentPattern = /^[A-Za-z0-9_.-]+$/;
const unknown = 'unknown';

export class ProjectVersionError extends Error {
  constructor(message: string, options?: ErrorOptions) {
    super(message, options);
    this.name = 'ProjectVersionError';
  }
}

function readProjectVersion(repositoryRoot: string): string {
  let version: string;

  try {
    version = readFileSync(resolve(repositoryRoot, 'VERSION'), 'utf8').trim();
  } catch (error) {
    throw new ProjectVersionError('Unable to read the repository VERSION file.', { cause: error });
  }

  if (!versionPattern.test(version)) {
    throw new ProjectVersionError('VERSION must use strict X.Y.Z format.');
  }

  const safeComponents = version
    .split('.')
    .map(Number)
    .every((value) => Number.isSafeInteger(value));

  if (!safeComponents) {
    throw new ProjectVersionError('Every VERSION component must be a JavaScript-safe integer.');
  }

  return version;
}

function nonEmpty(value: string | undefined): string {
  return value?.trim() || '';
}

function isoTimestamp(value: string): string {
  if (!value) {
    return '';
  }

  const timestamp = new Date(value);

  return Number.isNaN(timestamp.getTime()) ? '' : timestamp.toISOString();
}

export function githubCommitUrl(repositoryUrl: string, commit: string): string {
  if (!commitPattern.test(commit)) {
    return '';
  }

  const normalizedUrl = repositoryUrl
    .trim()
    .replace(/\/$/, '')
    .replace(/\.git$/, '');
  const prefixes = ['https://github.com/', 'git@github.com:', 'ssh://git@github.com/'] as const;
  const prefix = prefixes.find((candidate) => normalizedUrl.startsWith(candidate));

  if (!prefix) {
    return '';
  }

  const repositoryPath = normalizedUrl.slice(prefix.length);
  const segments = repositoryPath.split('/');
  const owner = segments[0];
  const repository = segments[1];

  if (
    segments.length !== 2 ||
    !owner ||
    !repository ||
    !githubSegmentPattern.test(owner) ||
    !githubSegmentPattern.test(repository)
  ) {
    return '';
  }

  return `https://github.com/${owner}/${repository}/commit/${commit}`;
}

export function resolveBuildMetadata({
  repositoryRoot,
  environment,
  now,
  runGit,
  warn,
}: BuildMetadataOptions): BuildMetadata {
  let warnedAboutGit = false;
  const gitValue = (args: readonly string[]): string => {
    try {
      return runGit(args).trim();
    } catch (error) {
      if (!warnedAboutGit) {
        warnedAboutGit = true;
        warn?.('Git metadata is unavailable; using build metadata fallbacks.');
      }

      if (error instanceof Error) {
        return '';
      }

      throw error;
    }
  };
  const explicitCommit = nonEmpty(environment.WEB_BUILD_COMMIT);
  const commit = explicitCommit || gitValue(['rev-parse', 'HEAD']) || unknown;
  const ref =
    nonEmpty(environment.WEB_BUILD_REF) ||
    gitValue(['rev-parse', '--abbrev-ref', 'HEAD']) ||
    unknown;
  const explicitCommitTime = nonEmpty(environment.WEB_BUILD_COMMIT_TIME);
  const commitTime = isoTimestamp(
    explicitCommitTime || gitValue(['log', '-1', '--pretty=format:%cI']),
  );
  const repositoryUrl =
    nonEmpty(environment.WEB_BUILD_REPOSITORY_URL) ||
    gitValue(['config', '--get', 'remote.origin.url']);
  const explicitBuildTime = isoTimestamp(nonEmpty(environment.WEB_BUILD_TIME));

  return {
    component,
    version: readProjectVersion(repositoryRoot),
    ref,
    commit,
    shortCommit: commit === unknown ? unknown : commit.slice(0, 9),
    commitTime,
    commitUrl: githubCommitUrl(repositoryUrl, commit),
    buildTime: explicitBuildTime || now.toISOString(),
  };
}

const integrationDirectory = dirname(fileURLToPath(import.meta.url));
const repositoryRoot = resolve(integrationDirectory, '../../../../..');

const runRepositoryGit = (args: readonly string[]): string =>
  execFileSync('git', args, {
    cwd: repositoryRoot,
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'pipe'],
  }).trim();

export function buildMetadataIntegration(): AstroIntegration {
  return {
    name: 'heyblog-build-metadata',
    hooks: {
      'astro:config:setup': ({ logger, updateConfig }) => {
        const metadata = resolveBuildMetadata({
          repositoryRoot,
          environment: process.env,
          now: new Date(),
          runGit: runRepositoryGit,
          warn: (message) => logger.warn(message),
        });

        updateConfig({
          vite: {
            define: {
              __HEYBLOG_BUILD_METADATA__: JSON.stringify(metadata),
            },
          },
        });
      },
    },
  };
}
