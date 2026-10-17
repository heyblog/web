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

import { parseSiteDirectorySearchParams } from '../src/application/site-directory/site-directory.shared.ts';

import { profile } from './site-og.fixture.ts';

test('development keeps page scripts and QR images working after graph workers load', async () => {
  // Given: a fresh dev server and an isolated API with public fixture data.
  const directory = await mkdtemp(join(tmpdir(), 'heyblog-dev-'));
  let emptyLists = false;
  let sessionStatus = 401;
  const oauthCallbacks: string[] = [];
  const passwordRequests: unknown[] = [];
  const api = createServer((request, response) => {
    response.setHeader('Content-Type', 'application/json');
    if (request.url === '/api/web/auth/me') {
      response.statusCode = sessionStatus;
      response.end(JSON.stringify({ code: 'session_unavailable' }));
    } else if (request.url === '/api/web/auth/password') {
      request.setEncoding('utf8');
      let body = '';
      request.on('data', (chunk: string) => {
        body += chunk;
      });
      request.on('end', () => {
        const payload: unknown = JSON.parse(body);
        passwordRequests.push(payload);
        if (
          !payload ||
          typeof payload !== 'object' ||
          !('current_password' in payload) ||
          typeof payload.current_password !== 'string'
        ) {
          response.statusCode = 422;
          response.end(JSON.stringify({ code: 'validation_failed' }));
          return;
        }
        response.setHeader('Set-Cookie', [
          'heyblog_access_token=renewed; Path=/; HttpOnly',
          'heyblog_refresh_token=renewed; Path=/; HttpOnly',
        ]);
        response.end(JSON.stringify({ user: {} }));
      });
    } else if (request.url === '/api/web/auth/logout') {
      response.statusCode = 204;
      response.setHeader('Set-Cookie', [
        'heyblog_access_token=; Max-Age=0; Path=/; HttpOnly',
        'heyblog_refresh_token=; Max-Age=0; Path=/; HttpOnly',
      ]);
      response.end();
    } else if (
      request.url?.startsWith('/api/web/auth/github/callback?') ||
      request.url?.startsWith('/api/web/auth/blogsclub/callback?')
    ) {
      oauthCallbacks.push(request.url);
      const callback = new URL(request.url, 'http://api.test');
      const provider = callback.pathname.includes('/github/') ? 'github' : 'blogsclub';
      response.setHeader('Set-Cookie', ['oauth-state=; Path=/; Max-Age=0']);
      if (callback.searchParams.get('code') === 'invalid') {
        response.statusCode = 400;
        response.end(
          JSON.stringify({ code: 'invalid_oauth_state', detail: 'private-oauth-detail' }),
        );
      } else {
        response.setHeader('Set-Cookie', [
          'oauth-state=; Path=/; Max-Age=0',
          'session=renewed; Path=/; HttpOnly',
        ]);
        response.end(
          JSON.stringify({
            provider,
            state: callback.searchParams.get('state'),
            intent: 'login',
            mode: callback.searchParams.get('code') === 'redirect' ? 'redirect' : 'popup',
            next: '/dashboard/security',
            outcome: 'success',
          }),
        );
      }
    } else if (
      request.url === `/api/web/public/sites/id/${profile.shortId}` ||
      request.url === `/api/web/public/sites/id/${profile.shortId}/metadata`
    ) {
      response.end(JSON.stringify(profile));
    } else if (request.url === '/api/web/public/sites/options') {
      response.end(
        JSON.stringify({ classifications: [], tertiaryTags: [], warnings: [], technologies: [] }),
      );
    } else if (request.url?.startsWith('/api/web/public/sites?')) {
      const query = parseSiteDirectorySearchParams(
        new URL(request.url, 'http://api.test').searchParams,
      );
      const totalPages = emptyLists ? 1 : 2;
      const page = Math.min(query.page, totalPages);
      response.end(
        JSON.stringify({
          items: [],
          pagination: { page, pageSize: 24, totalItems: emptyLists ? 0 : 48, totalPages },
          query: { ...query, page },
          statusCounts: { normal: emptyLists ? 0 : 48, abnormal: 0 },
        }),
      );
    } else if (request.url?.startsWith('/api/web/public/announcements?')) {
      const page = Number(new URL(request.url, 'http://api.test').searchParams.get('page'));
      response.end(
        JSON.stringify({ announcements: [], total: emptyLists ? 0 : 40, page, pageSize: 20 }),
      );
    } else if (request.url?.startsWith('/api/web/public/sitemap?')) {
      response.end(
        JSON.stringify({
          items: request.url.includes('kind=sites')
            ? [{ id: '019ded7f-4be3-7168-8953-b64109326083', shortId: profile.shortId }]
            : [],
          nextAfter: null,
        }),
      );
    } else if (request.url === '/api/web/public/home') {
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

    // Auth failures return native redirects with immutable headers.
    const failedLogin = await fetch(origin + '/api/web/auth/login', {
      method: 'POST',
      headers: { Origin: origin },
      body: new URLSearchParams({ identifier: 'synthetic', password: 'invalid' }),
      redirect: 'manual',
    });
    const failedOAuth = await fetch(origin + '/api/web/auth/github/start', { redirect: 'manual' });
    for (const [response, status] of [
      [failedLogin, 303],
      [failedOAuth, 302],
    ] as const) {
      assert.equal(response.status, status);
      assert.equal(response.headers.get('Location'), origin + '/login?error=unauthorized');
      assert.equal(response.headers.get('X-Robots-Tag'), 'noindex, follow');
      assert.equal(await response.text(), '');
    }
    const mismatch = await fetch(origin + '/api/web/auth/password/reset', {
      method: 'POST',
      headers: { Origin: origin },
      body: new URLSearchParams({
        token: 'synthetic-reset-token',
        password: 'new-password',
        confirm_password: 'different-password',
      }),
      redirect: 'manual',
    });
    assert.equal(mismatch.status, 303);
    const mismatchLocation = new URL(mismatch.headers.get('Location') ?? '', origin);
    assert.equal(mismatchLocation.pathname, '/reset-password');
    assert.equal(mismatchLocation.searchParams.get('error'), 'password_mismatch');
    assert.equal(mismatchLocation.searchParams.get('token'), 'synthetic-reset-token');
    assert.equal(await mismatch.text(), '');

    const logout = await fetch(origin + '/api/web/auth/logout', {
      method: 'POST',
      headers: { Origin: origin },
      body: new URLSearchParams(),
      redirect: 'manual',
    });
    assert.equal(logout.status, 303);
    assert.equal(logout.headers.get('Location'), '/login');
    assert.equal(logout.headers.get('Cache-Control'), 'private, no-store');
    assert.deepEqual(logout.headers.getSetCookie(), [
      'heyblog_access_token=; Max-Age=0; Path=/; HttpOnly',
      'heyblog_refresh_token=; Max-Age=0; Path=/; HttpOnly',
    ]);

    // Both public callbacks use the shared page and forward to their internal API endpoint.
    for (const provider of ['github', 'blogsclub']) {
      for (const code of ['oauth-test-secret-code', 'redirect', 'invalid']) {
        const query = new URLSearchParams({ code, state: `${provider}-state` });
        const response = await fetch(`${origin}/auth/${provider}/callback?${query}`, {
          redirect: 'manual',
        });
        assert.equal(response.status, code === 'redirect' ? 303 : 200);
        assert.equal(oauthCallbacks.at(-1), `/api/web/auth/${provider}/callback?${query}`);
        assert.equal(response.headers.get('Cache-Control'), 'private, no-store');
        assert.equal(response.headers.get('Referrer-Policy'), 'no-referrer');
        assert.match(response.headers.get('X-Robots-Tag') ?? '', /noindex/u);
        assert.deepEqual(response.headers.getSetCookie(), [
          'oauth-state=; Path=/; Max-Age=0',
          ...(code === 'invalid' ? [] : ['session=renewed; Path=/; HttpOnly']),
        ]);
        if (code === 'redirect') {
          assert.equal(response.headers.get('Location'), origin + '/dashboard/security');
        } else {
          const html = await response.text();
          assert.match(html, /data-oauth-result=/u);
          assert.ok(html.includes(code === 'invalid' ? '授权未完成' : '授权完成'));
          assert.ok(!html.includes('oauth-test-secret-code'));
          assert.ok(!html.includes('private-oauth-detail'));
        }
      }
    }
    const callbackCount = oauthCallbacks.length;
    for (const path of ['/auth/unknown/callback', '/api/web/auth/github/callback']) {
      const response = await fetch(origin + path, { redirect: 'manual' });
      assert.equal(response.status, 404, path);
      await response.text();
    }
    assert.equal(oauthCallbacks.length, callbackCount);

    // Given: an OAuth user has no current-password field on first setup.
    // When: the real form route forwards setup, or a password containing whitespace.
    for (const current of [undefined, ' current-password ']) {
      const next = ' new-password ';
      const body = new URLSearchParams({ next_password: next, confirm_password: next });
      if (current !== undefined) body.set('current_password', current);
      const response = await fetch(origin + '/api/web/auth/password', {
        method: 'POST',
        headers: { Origin: origin },
        body,
        redirect: 'manual',
      });
      // Then: the API receives strings unchanged, and renewed sessions reach the browser.
      assert.deepEqual(passwordRequests.at(-1), {
        current_password: current ?? '',
        next_password: next,
      });
      assert.equal(response.status, 303);
      assert.equal(response.headers.get('Location'), '/dashboard/security?status=password-updated');
      assert.equal(response.headers.getSetCookie().length, 2);
      assert.equal(response.headers.get('Cache-Control'), 'private, no-store');
    }

    // Given: the confirmation does not match the proposed password.
    // When: the browser submits the form.
    const passwordMismatch = await fetch(origin + '/api/web/auth/password', {
      method: 'POST',
      headers: { Origin: origin },
      body: new URLSearchParams({ next_password: 'new-password', confirm_password: 'different' }),
      redirect: 'manual',
    });
    // Then: validation redirects locally without sending another mutation upstream.
    assert.equal(passwordMismatch.status, 303);
    assert.equal(
      passwordMismatch.headers.get('Location'),
      origin + '/dashboard/security?error=password_mismatch',
    );
    assert.equal(passwordRequests.length, 2);

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

    // SEO metadata and XML must be visible in the actual server-rendered response.
    const detail = await get(`/site/${profile.shortId}`);
    const detailHtml = await detail.text();
    assert.equal(detail.status, 200);
    assert.ok(detailHtml.includes(`href="https://www.heyblog.net/site/${profile.shortId}"`));
    assert.ok(detailHtml.includes('property="og:site_name" content="HeyBlog"'));
    const json = detailHtml.match(
      /<script[^>]*type="application\/ld\+json"[^>]*>([\s\S]*?)<\/script>/,
    )?.[1];
    assert.ok(json, 'server-rendered JSON-LD');
    const graph: unknown = JSON.parse(json);
    assert.ok(JSON.stringify(graph).includes(profile.homepageUrl));
    assert.ok(!detailHtml.includes('article:published_time'));
    const sitemap = await get('/sitemap.xml');
    assert.equal(sitemap.status, 200);
    assert.match(sitemap.headers.get('content-type') ?? '', /application\/xml/);
    assert.match(await sitemap.text(), /sitemap-sites-1.xml/);
    const siteMap = await get('/sitemap-sites-1.xml');
    assert.ok((await siteMap.text()).includes(`https://www.heyblog.net/site/${profile.shortId}`));
    const staticMap = await get('/sitemap-static-1.xml');
    const staticXml = await staticMap.text();
    assert.match(staticXml, /\/blog\/2026081201/);
    assert.doesNotMatch(staticXml, /\/login|\/management|\/site\/go|\/site\/submissions/);
    const filtered = await get('/site/?q=test');
    assert.equal(filtered.status, 200);
    assert.match(filtered.headers.get('X-Robots-Tag') ?? '', /noindex/);
    const paged = await get('/site/?page=2');
    assert.equal(paged.status, 200);
    assert.equal(paged.headers.get('X-Robots-Tag'), null);
    assert.ok((await paged.text()).includes('href="https://www.heyblog.net/site?page=2"'));
    // Given: two public pages, including an API that clamps site requests to page 2.
    // When: a crawler requests a nonexistent page on either list.
    for (const path of ['/site/?page=999', '/announcements?page=999']) {
      const response = await get(path);
      const html = await response.text();
      // Then: the actual HTTP response and rendered metadata exclude that page from search.
      assert.equal(response.status, 404, path);
      assert.match(response.headers.get('X-Robots-Tag') ?? '', /noindex/, path);
      assert.match(html, /name="robots" content="noindex/, path);
      assert.doesNotMatch(html, /application\/ld\+json/, path);
    }
    const announcementPage = await get('/announcements?page=2');
    assert.equal(announcementPage.status, 200);
    assert.equal(announcementPage.headers.get('X-Robots-Tag'), null);
    assert.ok(
      (await announcementPage.text()).includes(
        'href="https://www.heyblog.net/announcements?page=2"',
      ),
    );
    emptyLists = true;
    for (const path of ['/site', '/announcements']) {
      const response = await get(path);
      assert.equal(response.status, 200, `empty first page: ${path}`);
      assert.equal(response.headers.get('X-Robots-Tag'), null, path);
      await response.text();
    }
    emptyLists = false;
    const login = await get('/login');
    const loginHtml = await login.text();
    assert.match(login.headers.get('X-Robots-Tag') ?? '', /noindex/);
    assert.match(loginHtml, /name="robots" content="noindex/);
    assert.doesNotMatch(loginHtml, /application\/ld\+json/);
    sessionStatus = 503;
    for (const path of ['/management/database-backup', '/dashboard', '/login']) {
      const response = await fetch(origin + path, {
        headers: { Cookie: 'heyblog_access_token=current' },
        redirect: 'manual',
      });
      assert.equal(response.status, 503, path);
      assert.equal(response.headers.get('Location'), null);
      assert.equal(response.headers.get('Set-Cookie'), null);
      assert.equal(response.headers.get('Cache-Control'), 'private, no-store');
      assert.equal(response.headers.get('Retry-After'), '5');
      assert.equal(await response.text(), '暂时无法读取登录状态，请稍后刷新重试。');
    }
    sessionStatus = 401;
    const alias = await fetch(origin + '/sitemap-index.xml', { redirect: 'manual' });
    assert.equal(alias.status, 308);
    assert.equal(alias.headers.get('Location'), '/sitemap.xml');
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
