import assert from 'node:assert/strict';
import { fork } from 'node:child_process';
import { once } from 'node:events';
import { cp, mkdtemp, rm } from 'node:fs/promises';
import { createServer } from 'node:http';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import process from 'node:process';
import { pathToFileURL } from 'node:url';
import test from 'node:test';

if (process.argv[2] === '--child') {
  const { startServer } = await import('./dist/server/entry.mjs');
  const running = startServer();
  await once(running.server.server, 'listening');
  process.send({ port: running.server.server.address().port });
  await once(process, 'message');
  await running.server.stop();
  process.disconnect();
} else {
  test('isolated dist serves site metadata and PNGs with ID redirects and failure statuses', async () => {
    const { profile } = await import('./tests/site-og.fixture.ts');
    const { PNG } = await import('pngjs');
    const { default: jsQR } = await import('jsqr');
    // Given: only dist is deployed, with synthetic public-site API responses.
    const directory = await mkdtemp(join(tmpdir(), 'heyblog-og-'));
    const uuid = '019ded7f-4be3-7168-8953-b64109326083';
    let current = profile;
    let iconStatus = 200;
    const icon = new PNG({ width: 32, height: 32 });
    for (let index = 0; index < icon.data.length; index += 4)
      icon.data.set([230, 80, 30, 255], index);
    const iconBytes = PNG.sync.write(icon);
    const requested = [];
    const api = createServer((request, response) => {
      requested.push(request.url);
      response.setHeader('Content-Type', 'application/json');
      assert.equal(
        request.headers['x-heyblog-web-token'],
        'test-web-service-token-0123456789abcdef',
      );
      assert.equal(request.headers.cookie, undefined);
      if (request.url === `/sites/id/${profile.shortId}/icon`) {
        response.statusCode = iconStatus;
        response.setHeader('Content-Type', 'image/png');
        response.setHeader('ETag', `"${current.iconHash}"`);
        response.end(iconStatus === 200 ? iconBytes : undefined);
      } else if (
        [`/sites/id/${profile.shortId}`, `/sites/id/${uuid}`, '/sites/custom/wuke'].includes(
          request.url,
        )
      ) {
        response.end(JSON.stringify(current));
      } else {
        response.statusCode = request.url === '/sites/id/999999999' ? 503 : 404;
        response.end('{}');
      }
    });
    let child;
    let stopped;
    try {
      await cp(new URL('./dist/', import.meta.url), join(directory, 'dist'), { recursive: true });
      await cp(new URL(import.meta.url), join(directory, 'runner.mjs'));
      api.listen(0, '127.0.0.1');
      await once(api, 'listening');
      child = fork(pathToFileURL(join(directory, 'runner.mjs')), ['--child'], {
        cwd: directory,
        env: {
          NODE_ENV: 'production',
          ASTRO_NODE_AUTOSTART: 'disabled',
          HOST: '127.0.0.1',
          PORT: '0',
          API_WEB_TOKEN: 'test-web-service-token-0123456789abcdef',
          WEB_API_BASE_URL: `http://127.0.0.1:${api.address().port}`,
        },
        silent: true,
      });
      stopped = once(child, 'exit');
      const [ready] = await Promise.race([
        once(child, 'message', { signal: AbortSignal.timeout(15_000) }),
        stopped.then(([code]) => {
          throw new Error(`Isolated server exited early: ${code}`);
        }),
      ]);
      const origin = `http://127.0.0.1:${ready.port}`;
      const get = (path) =>
        fetch(`${origin}${path}`, { redirect: 'manual', signal: AbortSignal.timeout(15_000) });
      const imagePath = (html) =>
        html.match(/property="og:image" content="[^"]+?(\/og\/site\/[^" ]+)"/u)?.[1];

      // When: a crawler requests a site's real HTML and follows its image URL.
      const page = await get(`/site/${profile.shortId}`);
      const html = await page.text();
      assert.equal(page.status, 200);
      const title = `${profile.name} | HeyBlog`;
      assert.ok(html.includes(`<title>${title}</title>`));
      for (const property of ['og:title', 'og:site_name']) {
        assert.ok(html.includes(`property="${property}" content="${title}"`));
      }
      assert.ok(html.includes(`name="twitter:title" content="${title}"`));
      for (const key of ['description', 'twitter:description']) {
        assert.ok(html.includes(`name="${key}" content="${profile.summary}"`));
      }
      assert.ok(html.includes(`property="og:description" content="${profile.summary}"`));
      assert.ok(
        html.includes(`rel="canonical" href="https://www.heyblog.net/site/${profile.shortId}"`),
      );
      assert.ok(html.includes('property="og:image:type" content="image/png"'));
      assert.ok(html.includes('property="og:image:width" content="1200"'));
      assert.ok(html.includes('property="og:image:height" content="630"'));
      const path = imagePath(html);
      assert.ok(path);
      const image = await get(path);
      const bytes = new Uint8Array(await image.arrayBuffer());

      // Then: the deployed image renders without system fonts or node_modules.
      assert.equal(image.status, 200);
      assert.equal(image.headers.get('content-type'), 'image/png');
      assert.match(image.headers.get('cache-control'), /s-maxage=300/u);
      assert.deepEqual([...bytes.slice(0, 8)], [137, 80, 78, 71, 13, 10, 26, 10]);
      const png = new DataView(bytes.buffer);
      assert.equal(png.getUint32(16), 1200);
      assert.equal(png.getUint32(20), 630);
      assert.ok(bytes.length > 10_000, 'Card includes rendered text and graphics');
      const pixels = PNG.sync.read(Buffer.from(bytes));
      assert.equal(
        jsQR(new Uint8ClampedArray(pixels.data), pixels.width, pixels.height)?.data,
        `https://www.heyblog.net/site/${profile.shortId}`,
      );

      for (const [source, target] of [
        [`/site/${uuid}`, `/site/${profile.shortId}`],
        ['/s/wuke', `/site/${profile.shortId}`],
        [`/og/site/${uuid}.png`, path],
      ]) {
        const redirect = await get(source);
        assert.equal(redirect.status, 308);
        assert.equal(redirect.headers.get('location'), target);
      }
      const before = requested.length;
      assert.equal((await get('/og/site/wuke.png')).status, 404);
      assert.equal(requested.length, before, 'Custom ID is not looked up by the image route');
      assert.equal((await get('/og/site/UnknownID.png')).status, 404);
      assert.ok(
        requested.includes('/sites/id/UnknownID'),
        'Nine-character values follow the short-ID lookup, not custom-ID lookup',
      );
      for (const [identifier, status] of [
        ['000000000', 404],
        ['999999999', 503],
      ]) {
        for (const route of [`/site/${identifier}`, `/og/site/${identifier}.png`]) {
          const response = await get(route);
          assert.equal(response.status, status);
          assert.equal(response.headers.get('cache-control'), 'no-store');
          if (status === 503) assert.equal(response.headers.get('retry-after'), '30');
          if (route.startsWith('/site/')) assert.match(await response.text(), /noindex, nofollow/u);
        }
      }
      current = { ...profile, summary: '更新后的站点简介', classification: null };
      const updated = imagePath(await (await get(`/site/${profile.shortId}`)).text());
      assert.notEqual(updated, path);
      assert.notDeepEqual(new Uint8Array(await (await get(updated)).arrayBuffer()), bytes);
      current = { ...profile, summary: ' ' };
      const emptyHtml = await (await get(`/site/${profile.shortId}`)).text();
      assert.ok(emptyHtml.includes(`${profile.name}（${profile.host}）的博客收录信息。`));
      assert.equal((await get('/licenses/noto-sans-sc.txt')).status, 200);
      current = { ...profile, iconHash: 'a'.repeat(64) };
      const iconPath = imagePath(await (await get(`/site/${profile.shortId}`)).text());
      const iconResponse = await get(iconPath);
      const withIcon = new Uint8Array(await iconResponse.arrayBuffer());
      assert.equal(iconResponse.status, 200);
      assert.notEqual(iconPath, path);
      assert.notDeepEqual(bytes, withIcon);
      assert.ok(requested.includes(`/sites/id/${profile.shortId}/icon`));
      iconStatus = 503;
      const fallback = await get(iconPath);
      assert.equal(fallback.status, 200);
      assert.equal(fallback.headers.get('cache-control'), 'no-store');
      assert.deepEqual(new Uint8Array(await fallback.arrayBuffer()), bytes);
    } finally {
      if (child && child.exitCode === null && child.signalCode === null) {
        const forceStop = setTimeout(() => child.kill('SIGKILL'), 5_000);
        if (child.connected) child.send({ kind: 'stop' });
        else child.kill('SIGTERM');
        await stopped;
        clearTimeout(forceStop);
      }
      if (api.listening)
        await new Promise((resolve, reject) =>
          api.close((error) => (error ? reject(error) : resolve())),
        );
      await rm(directory, { recursive: true, force: true });
    }
  });
}
