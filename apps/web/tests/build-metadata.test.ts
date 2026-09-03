import assert from 'node:assert/strict';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import test from 'node:test';

import {
  githubCommitUrl,
  ProjectVersionError,
  resolveBuildMetadata,
} from '../src/shared/integrations/build-metadata.ts';

const repositoryRoot = resolve(import.meta.dirname, '../../..');
const fixedBuildTime = new Date('2026-09-03T02:03:04.000Z');

const gitValues = new Map<string, string>([
  ['rev-parse HEAD', '1111111111111111111111111111111111111111'],
  ['rev-parse --abbrev-ref HEAD', 'git-fallback'],
  ['log -1 --pretty=format:%cI', '2026-08-30T10:20:30+08:00'],
  ['config --get remote.origin.url', 'git@github.com:example/fallback.git'],
]);

const runGit = (args: readonly string[]): string => {
  const value = gitValues.get(args.join(' '));

  if (!value) {
    throw new RangeError(`Unexpected Git command: ${args.join(' ')}`);
  }

  return value;
};

test('reads the release version from the repository VERSION file', () => {
  // Given: the checked-out repository and explicit build metadata.
  const environment = {
    WEB_BUILD_COMMIT: '2222222222222222222222222222222222222222',
    WEB_BUILD_REF: 'refs/heads/release',
    WEB_BUILD_COMMIT_TIME: '2026-09-01T01:02:03.000Z',
    WEB_BUILD_REPOSITORY_URL: 'https://github.com/heyblog/heyblog.git',
    WEB_BUILD_TIME: '2026-09-03T01:02:03.000Z',
  } as const;

  // When: build metadata is resolved.
  const metadata = resolveBuildMetadata({
    repositoryRoot,
    environment,
    now: fixedBuildTime,
    runGit,
  });

  // Then: VERSION remains the only release-version source.
  assert.equal(metadata.version, '0.1.5');
});

test('prefers explicit build inputs over local Git metadata', () => {
  // Given: explicit inputs that differ from every Git fallback.
  const environment = {
    WEB_BUILD_COMMIT: '2222222222222222222222222222222222222222',
    WEB_BUILD_REF: 'refs/heads/release',
    WEB_BUILD_COMMIT_TIME: '2026-09-01T01:02:03.000Z',
    WEB_BUILD_REPOSITORY_URL: 'https://github.com/heyblog/heyblog.git',
    WEB_BUILD_TIME: '2026-09-03T01:02:03.000Z',
  } as const;

  // When: build metadata is resolved.
  const metadata = resolveBuildMetadata({
    repositoryRoot,
    environment,
    now: fixedBuildTime,
    runGit,
  });

  // Then: the explicit build provenance wins.
  assert.deepEqual(metadata, {
    component: 'heyblog-web',
    version: '0.1.5',
    ref: 'refs/heads/release',
    commit: '2222222222222222222222222222222222222222',
    shortCommit: '222222222',
    commitTime: '2026-09-01T01:02:03.000Z',
    commitUrl: 'https://github.com/heyblog/heyblog/commit/2222222222222222222222222222222222222222',
    buildTime: '2026-09-03T01:02:03.000Z',
  });
});

test('falls back to local Git metadata when build inputs are absent', () => {
  // Given: no explicit build provenance.
  const environment = {};

  // When: build metadata is resolved from Git.
  const metadata = resolveBuildMetadata({
    repositoryRoot,
    environment,
    now: fixedBuildTime,
    runGit,
  });

  // Then: Git and the injected clock supply the missing fields.
  assert.deepEqual(metadata, {
    component: 'heyblog-web',
    version: '0.1.5',
    ref: 'git-fallback',
    commit: '1111111111111111111111111111111111111111',
    shortCommit: '111111111',
    commitTime: '2026-08-30T02:20:30.000Z',
    commitUrl:
      'https://github.com/example/fallback/commit/1111111111111111111111111111111111111111',
    buildTime: '2026-09-03T02:03:04.000Z',
  });
});

test('creates commit links only for supported GitHub repository URLs', () => {
  // Given: HTTPS, SCP-style SSH, URL-style SSH, and untrusted repository URLs.
  const commit = 'abcdef1234567890';

  // When/Then: supported GitHub forms map to canonical HTTPS commit links.
  assert.equal(
    githubCommitUrl('https://github.com/heyblog/heyblog.git', commit),
    `https://github.com/heyblog/heyblog/commit/${commit}`,
  );
  assert.equal(
    githubCommitUrl('git@github.com:heyblog/heyblog.git', commit),
    `https://github.com/heyblog/heyblog/commit/${commit}`,
  );
  assert.equal(
    githubCommitUrl('ssh://git@github.com/heyblog/heyblog.git', commit),
    `https://github.com/heyblog/heyblog/commit/${commit}`,
  );
  assert.equal(githubCommitUrl('https://github.com.example/heyblog/heyblog.git', commit), '');
  assert.equal(githubCommitUrl('https://gitlab.com/heyblog/heyblog.git', commit), '');
});

test('uses safe fallbacks when Git metadata is unavailable', () => {
  // Given: a source archive without Git metadata.
  const warnings: string[] = [];
  const unavailableGit = (): string => {
    throw new Error('not a Git checkout');
  };

  // When: metadata resolution cannot invoke Git.
  const metadata = resolveBuildMetadata({
    repositoryRoot,
    environment: {},
    now: fixedBuildTime,
    runGit: unavailableGit,
    warn: (message) => warnings.push(message),
  });

  // Then: the build remains usable without inventing provenance.
  assert.equal(metadata.ref, 'unknown');
  assert.equal(metadata.commit, 'unknown');
  assert.equal(metadata.shortCommit, 'unknown');
  assert.equal(metadata.commitTime, '');
  assert.equal(metadata.commitUrl, '');
  assert.equal(warnings.length, 1);
});

test('fails when VERSION is missing or invalid', () => {
  // Given: temporary repository roots with missing and malformed VERSION files.
  const missingRoot = mkdtempSync(join(tmpdir(), 'heyblog-version-missing-'));
  const invalidRoot = mkdtempSync(join(tmpdir(), 'heyblog-version-invalid-'));
  writeFileSync(join(invalidRoot, 'VERSION'), '1.2\n', 'utf8');

  try {
    // When/Then: neither invalid source can produce build metadata.
    assert.throws(
      () =>
        resolveBuildMetadata({
          repositoryRoot: missingRoot,
          environment: {},
          now: fixedBuildTime,
          runGit,
        }),
      ProjectVersionError,
    );
    assert.throws(
      () =>
        resolveBuildMetadata({
          repositoryRoot: invalidRoot,
          environment: {},
          now: fixedBuildTime,
          runGit,
        }),
      ProjectVersionError,
    );
  } finally {
    rmSync(missingRoot, { recursive: true, force: true });
    rmSync(invalidRoot, { recursive: true, force: true });
  }
});
