import assert from 'node:assert/strict';
import { once } from 'node:events';
import { mkdtemp, rm } from 'node:fs/promises';
import { createServer } from 'node:http';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

import { dev } from 'astro';

test('management pages render taxonomy DTOs privately and enforce role permissions', async () => {
  const directory = await mkdtemp(join(tmpdir(), 'heyblog-taxonomy-pages-'));
  let role = 'SYS_ADMIN';
  let permissions: string[] = [];
  const tagID = '019f033c-2111-7000-9000-000000000001';
  const api = createServer((request, response) => {
    response.setHeader('Content-Type', 'application/json');
    if (request.url === '/auth/me')
      response.end(
        JSON.stringify({
          user: {
            id: tagID,
            role,
            permissions,
            display_name: '测试管理员',
            email: 'admin@example.test',
          },
        }),
      );
    else if (request.url === '/management/taxonomy/tags')
      response.end(
        JSON.stringify({
          tags: [
            {
              id: tagID,
              name: '中文标签',
              slug: 'chinese-tag',
              description: '',
              roles: ['TERTIARY'],
              is_enabled: true,
              site_count: 2,
              article_count: 1,
            },
          ],
          cascades: [],
          revision: 'a'.repeat(64),
        }),
      );
    else if (request.url === '/management/system-settings')
      response.end(
        JSON.stringify({ model_id: 'deepseek/deepseek-flash', revision: '0', configured: false }),
      );
    else {
      response.statusCode = 404;
      response.end('{}');
    }
  });
  let server: Awaited<ReturnType<typeof dev>> | undefined;
  try {
    api.listen(0, '127.0.0.1');
    await once(api, 'listening');
    const address = api.address();
    assert.ok(address && typeof address === 'object');
    process.env.WEB_API_BASE_URL = `http://127.0.0.1:${address.port}`;
    process.env.API_WEB_TOKEN = 'test-web-service-token-0123456789abcdef';
    process.env.WEB_BUILD_VERSION = '0.0.0';
    process.env.GITHUB_TOKEN = '';
    server = await dev({
      root: fileURLToPath(new URL('../', import.meta.url)),
      logLevel: 'silent',
      server: { host: '127.0.0.1', port: 0 },
      vite: { cacheDir: join(directory, 'vite') },
    });
    assert.ok(typeof server.address === 'object');
    const origin = `http://127.0.0.1:${server.address.port}`;
    const get = (path: string, authenticated = true) =>
      fetch(origin + path, {
        redirect: 'manual',
        headers: authenticated ? { Cookie: 'heyblog_access_token=test' } : {},
        signal: AbortSignal.timeout(120_000),
      });
    const tags = await get('/management/tags');
    assert.equal(tags.status, 200);
    assert.equal(tags.headers.get('Cache-Control'), 'private, no-store');
    const html = await tags.text();
    assert.match(html, /中文标签/u);
    assert.match(html, /新建标签/u);
    assert.match(html, /chinese-tag/u);
    assert.doesNotMatch(html, /test-web-service-token|tokenhub\.tencentmaas|API_TOKENHUB_API_KEY/u);
    const settings = await get('/management/system-settings');
    assert.equal(settings.status, 200);
    assert.equal(settings.headers.get('Cache-Control'), 'private, no-store');
    assert.match(await settings.text(), /DeepSeek-V4\.1-Flash/u);
    assert.equal(
      (await get('/management/tags', false)).headers.get('Location'),
      '/login?next=%2Fmanagement%2Ftags',
    );
    role = 'ADMIN';
    assert.equal((await get('/management/tags')).headers.get('Location'), '/forbidden');
    permissions = ['taxonomy.manage'];
    assert.equal((await get('/management/tags')).status, 200);
    assert.equal((await get('/management/system-settings')).headers.get('Location'), '/forbidden');
  } finally {
    await server?.stop();
    if (api.listening)
      await new Promise<void>((resolve, reject) =>
        api.close((error) => (error ? reject(error) : resolve())),
      );
    await rm(directory, { recursive: true, force: true });
  }
});
