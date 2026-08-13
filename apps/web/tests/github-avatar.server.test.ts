import assert from 'node:assert/strict';
import test from 'node:test';

import { createGithubAvatarHandler } from '../src/application/github-avatar/github-avatar.server.ts';
import { getGithubAvatarPath } from '../src/application/github-avatar/github-avatar.ts';

const avatarBytes = new Uint8Array([137, 80, 78, 71, 13, 10, 26, 10]);
const avatarRequest = (etag?: string) =>
  new Request('https://www.heyblog.net/media/github-avatar/GeonasZ.png', {
    headers: etag ? { 'If-None-Match': etag } : undefined,
  });

test('builds a same-origin avatar path only for valid GitHub usernames', () => {
  assert.equal(getGithubAvatarPath('GeonasZ'), '/media/github-avatar/GeonasZ.png');
  assert.throws(() => getGithubAvatarPath('../avatar'), /Invalid GitHub username/u);
});

test('caches GitHub avatars and supports browser conditional requests', async () => {
  let fetchCalls = 0;
  const handleAvatar = createGithubAvatarHandler({
    fetch: (async (input) => {
      fetchCalls += 1;
      assert.equal(String(input), 'https://github.com/GeonasZ.png?size=96');
      return new Response(avatarBytes, {
        headers: {
          'Content-Type': 'image/png',
          ETag: '"upstream-avatar"',
          'Last-Modified': 'Tue, 11 Aug 2026 12:00:00 GMT',
        },
      });
    }) as typeof fetch,
  });

  const first = await handleAvatar('GeonasZ', avatarRequest());
  const etag = first.headers.get('ETag');

  assert.equal(first.status, 200);
  assert.equal(first.headers.get('Content-Type'), 'image/png');
  assert.equal(first.headers.get('X-HeyBlog-Cache'), 'miss');
  assert.match(first.headers.get('Cache-Control') ?? '', /s-maxage=604800/u);
  assert.deepEqual(new Uint8Array(await first.arrayBuffer()), avatarBytes);
  assert.ok(etag);

  const conditional = await handleAvatar('geonasz', avatarRequest(etag ?? undefined));

  assert.equal(conditional.status, 304);
  assert.equal(conditional.headers.get('X-HeyBlog-Cache'), 'hit');
  assert.equal(fetchCalls, 1);
});

test('revalidates expired avatars and serves stale data when GitHub is unavailable', async () => {
  let currentTime = 0;
  let fetchCalls = 0;
  const handleAvatar = createGithubAvatarHandler({
    now: () => currentTime,
    freshTtlMs: 100,
    staleTtlMs: 1_000,
    fetch: (async (_input, init) => {
      fetchCalls += 1;

      if (fetchCalls === 1) {
        return new Response(avatarBytes, {
          headers: { 'Content-Type': 'image/png', ETag: '"upstream-avatar"' },
        });
      }

      assert.equal(new Headers(init?.headers).get('If-None-Match'), '"upstream-avatar"');
      if (fetchCalls === 2) {
        return new Response(null, { status: 304 });
      }

      throw new Error('GitHub unavailable');
    }) as typeof fetch,
  });

  await handleAvatar('GeonasZ', avatarRequest());
  currentTime = 101;
  const revalidated = await handleAvatar('GeonasZ', avatarRequest());

  assert.equal(revalidated.status, 200);
  assert.equal(revalidated.headers.get('X-HeyBlog-Cache'), 'revalidated');

  currentTime = 202;
  const stale = await handleAvatar('GeonasZ', avatarRequest());

  assert.equal(stale.status, 200);
  assert.equal(stale.headers.get('X-HeyBlog-Cache'), 'stale');
  assert.equal(stale.headers.get('Warning'), '110 - "Response is stale"');
  assert.equal(fetchCalls, 3);
});

test('rejects invalid names and unsafe upstream responses without caching them', async () => {
  let fetchCalls = 0;
  const handleAvatar = createGithubAvatarHandler({
    fetch: (async () => {
      fetchCalls += 1;
      return new Response('<html>not an image</html>', {
        headers: { 'Content-Type': 'text/html' },
      });
    }) as typeof fetch,
  });

  const invalid = await handleAvatar('../avatar', avatarRequest());
  const unsafe = await handleAvatar('GeonasZ', avatarRequest());

  assert.equal(invalid.status, 400);
  assert.equal(invalid.headers.get('Cache-Control'), 'no-store');
  assert.equal(unsafe.status, 502);
  assert.equal(unsafe.headers.get('Cache-Control'), 'no-store');
  assert.equal(fetchCalls, 1);
});
