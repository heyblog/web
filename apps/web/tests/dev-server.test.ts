import assert from 'node:assert/strict';
import { once } from 'node:events';
import { mkdtemp, rm } from 'node:fs/promises';
import { createServer } from 'node:http';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

import { dev } from 'astro';
import jsQR from 'jsqr';
import { PNG } from 'pngjs';

import { profile } from './site-og.fixture.ts';

test('development keeps page scripts and QR images working after graph workers load', async () => {
  // Given: a fresh dev server and an isolated API with public fixture data.
  const directory = await mkdtemp(join(tmpdir(), 'heyblog-dev-'));
  const api = createServer((request, response) => {
    response.setHeader('Content-Type', 'application/json');
    if (request.url === `/sites/id/${profile.shortId}`) {
      response.end(JSON.stringify(profile));
    } else if (request.url === '/home') {
      response.end(JSON.stringify({ siteCount: 0, announcements: [], sites: [] }));
    } else {
      response.statusCode = 401;
      response.end(JSON.stringify({ code: 'unauthorized' }));
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
    const get = (path: string) => fetch(origin + path, { signal: AbortSignal.timeout(120_000) });

    // Resolve the toolbar before late-loaded Worker imports can change the dependency bundle.
    const toolbar = '/@id/astro/runtime/client/dev-toolbar/entrypoint.js';
    const initialToolbar = await get(toolbar);
    assert.equal(initialToolbar.status, 200, 'initial development toolbar');
    await initialToolbar.text();
    for (const path of [
      '/src/application/site-graph/site-graph.layout.worker.ts?worker_file&type=module',
      '/src/application/site-graph/site-graph.engine.ts',
    ]) {
      const worker = await get(path);
      assert.equal(worker.status, 200, path);
      const source = await worker.text();
      let loadedDependencies = 0;
      for (const match of source.matchAll(/\bfrom\s+["']([^"']+)["']/gu)) {
        const dependency = match[1];
        if (!dependency?.startsWith('/') || !dependency.includes('/deps/')) continue;
        const optimized = await get(dependency);
        assert.equal(optimized.status, 200, dependency);
        await optimized.text();
        loadedDependencies++;
      }
      assert.ok(loadedDependencies > 0, `optimized Worker imports in ${path}`);
    }
    const loadedToolbar = await get(toolbar);
    assert.equal(loadedToolbar.status, 200, 'toolbar after graph Worker dependencies load');
    await loadedToolbar.text();

    // When: public prerendered and SSR pages share the development runtime.
    for (const path of ['/terms', '/privacy', '/blog', '/docs', '/', '/login']) {
      const response = await get(path);
      const html = await response.text();
      // Then: scripts resolve through the dev environment for every page.
      assert.equal(response.status, 200, path);
      assert.match(html, /PublicHeader\.astro/u, path);
    }

    const image = await get(`/og/site/${profile.shortId}.png`);
    assert.equal(image.status, 200);
    assert.equal(image.headers.get('content-type'), 'image/png');
    const pixels = PNG.sync.read(Buffer.from(await image.arrayBuffer()));
    assert.equal(
      jsQR(new Uint8ClampedArray(pixels.data), pixels.width, pixels.height)?.data,
      `https://www.heyblog.net/site/${profile.shortId}`,
    );
  } finally {
    await server?.stop();
    if (api.listening) {
      await new Promise<void>((resolve, reject) =>
        api.close((error) => (error ? reject(error) : resolve())),
      );
    }
    await rm(directory, { recursive: true, force: true });
  }
});
