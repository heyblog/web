import assert from 'node:assert/strict';
import test from 'node:test';

import {
  createGithubContributorsReader,
  isEligibleRepository,
} from '../src/application/github-contributors/github-contributors.ts';

const repository = (name: string, overrides: Record<string, unknown> = {}) => ({
  name,
  archived: false,
  fork: false,
  topics: [],
  ...overrides,
});

test('filters archived, forked, and legacy repositories', () => {
  assert.equal(isEligibleRepository(repository('web')), true);
  assert.equal(isEligibleRepository(repository('V1')), false);
  assert.equal(isEligibleRepository(repository('blog-daohang')), false);
  assert.equal(isEligibleRepository(repository('old', { archived: true })), false);
  assert.equal(isEligibleRepository(repository('fork', { fork: true })), false);
  assert.equal(isEligibleRepository(repository('legacy', { topics: ['zhblogs'] })), false);
});

test('fetches repositories and contributors with pagination, token, and aggregation', async () => {
  const calls: Array<{ url: string; authorization: string | null }> = [];
  const fetchGithub = (async (input, init) => {
    const url = String(input);
    calls.push({ url, authorization: new Headers(init?.headers).get('Authorization') });

    if (url.includes('/orgs/heyblog/repos?')) {
      if (url.endsWith('page=1')) {
        return new Response(
          JSON.stringify([
            repository('web'),
            repository('V1'),
            repository('old', { topics: ['zhblogs'] }),
          ]),
          { headers: { 'Content-Type': 'application/json' } },
        );
      }
      return new Response(JSON.stringify([]), { headers: { 'Content-Type': 'application/json' } });
    }

    if (url.includes('/repos/heyblog/web/contributors?') && url.endsWith('page=1')) {
      return new Response(
        JSON.stringify([
          {
            login: 'Alice',
            avatar_url: 'https://avatars.example/alice.png',
            html_url: 'https://github.com/Alice',
            contributions: 3,
          },
          {
            login: 'alice',
            avatar_url: 'https://avatars.example/alice.png',
            html_url: 'https://github.com/alice',
            contributions: 2,
          },
          {
            login: 'Bot',
            avatar_url: 'https://avatars.example/bot.png',
            html_url: 'https://github.com/apps/bot',
            contributions: 99,
            type: 'Bot',
          },
        ]),
        { headers: { 'Content-Type': 'application/json' } },
      );
    }

    throw new Error(`Unexpected URL: ${url}`);
  }) as typeof fetch;

  const read = createGithubContributorsReader({
    token: 'github-token',
    fetch: fetchGithub,
    freshTtlMs: 100,
    staleTtlMs: 1_000,
    timeoutMs: 1_000,
  });
  const snapshot = await read();

  assert.deepEqual(snapshot?.contributors, [
    {
      login: 'Alice',
      avatarUrl: 'https://avatars.example/alice.png',
      htmlUrl: 'https://github.com/Alice',
      contributions: 5,
      repositoryCount: 2,
    },
  ]);
  assert.ok(calls.length >= 2);
  assert.ok(calls.every((call) => call.authorization === 'Bearer github-token'));
});

test('serves stale cache after GitHub becomes unavailable and coalesces requests', async () => {
  let now = 0;
  let calls = 0;
  let available = true;
  const fetchGithub = (async (input) => {
    calls += 1;
    if (!available) throw new Error('unavailable');
    const url = String(input);
    if (url.includes('/orgs/heyblog/repos?')) {
      return new Response(JSON.stringify([repository('web')]), {
        headers: { 'Content-Type': 'application/json' },
      });
    }
    return new Response(
      JSON.stringify([
        {
          login: 'Alice',
          avatar_url: 'https://avatars.example/alice.png',
          html_url: 'https://github.com/Alice',
          contributions: 1,
        },
      ]),
      { headers: { 'Content-Type': 'application/json' } },
    );
  }) as typeof fetch;
  const read = createGithubContributorsReader({
    fetch: fetchGithub,
    now: () => now,
    freshTtlMs: 100,
    staleTtlMs: 1_000,
  });

  const first = await Promise.all([read(), read()]);
  assert.equal(first[0]?.contributors[0]?.login, 'Alice');
  assert.equal(calls, 2);

  now = 101;
  available = false;
  const stale = await read();
  assert.equal(stale?.stale, true);
  assert.equal(stale?.contributors[0]?.login, 'Alice');

  now = 1_101;
  assert.equal(await read(), undefined);
});
