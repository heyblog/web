import assert from 'node:assert/strict';
import { fork } from 'node:child_process';
import { createHash } from 'node:crypto';
import { once } from 'node:events';
import { createServer } from 'node:http';
import process from 'node:process';
import test from 'node:test';

const childMode = process.env.HEYBLOG_STANDALONE_SMOKE_CHILD === '1';

if (childMode) {
  process.env.ASTRO_NODE_AUTOSTART = 'disabled';
  process.env.HOST = '127.0.0.1';
  process.env.PORT = '0';

  const { startServer } = await import('./dist/server/entry.mjs');
  const runningServer = startServer();
  await once(runningServer.server.server, 'listening');

  const address = runningServer.server.server.address();
  assert.notEqual(address, null);
  assert.equal(typeof address, 'object');
  process.send?.({ kind: 'ready', port: address.port });

  const [message] = await once(process, 'message');
  assert.deepEqual(message, { kind: 'stop' });
  await runningServer.server.stop();
  process.disconnect();
} else {
  test('standalone server limits web analytics to non-sensitive pages', async () => {
    // Given: an authenticated API and production build started on ephemeral ports.
    const sessionUser = {
      id: 'test-admin',
      email: 'admin@example.test',
      username: 'admin',
      display_name: 'Test Admin',
      avatar_url: null,
      role: 'SYS_ADMIN',
      permissions: [],
      active: true,
      email_verified: true,
      has_password: true,
      has_github: false,
      auth_version: 1,
      created_at: '2026-09-18T00:00:00Z',
      last_login_at: null,
    };
    const apiServer = createServer((request, response) => {
      response.setHeader('Content-Type', 'application/json');

      if (request.url === '/home') {
        response.end(JSON.stringify({ siteCount: 0, announcement: null, sites: [] }));
        return;
      }
      if (request.url === '/auth/me') {
        response.end(JSON.stringify({ user: sessionUser }));
        return;
      }
      if (request.url === '/management/users') {
        response.end(JSON.stringify({ users: [sessionUser] }));
        return;
      }

      response.statusCode = 404;
      response.end(JSON.stringify({ code: 'not_found' }));
    });
    apiServer.listen(0, '127.0.0.1');
    await once(apiServer, 'listening');
    const apiAddress = apiServer.address();
    assert.notEqual(apiAddress, null);
    assert.equal(typeof apiAddress, 'object');

    const child = fork(new URL(import.meta.url), [], {
      env: {
        ...process.env,
        API_WEB_TOKEN: 'test-web-service-token-0123456789abcdef',
        HEYBLOG_STANDALONE_SMOKE_CHILD: '1',
        WEB_API_BASE_URL: `http://127.0.0.1:${apiAddress.port}`,
      },
      silent: true,
    });
    let stderr = '';
    child.stderr?.setEncoding('utf8');
    child.stderr?.on('data', (chunk) => {
      stderr += chunk;
    });

    try {
      const readyMessage = await new Promise((resolve, reject) => {
        const timeout = setTimeout(() => {
          child.kill('SIGTERM');
          reject(new Error('Standalone server did not report readiness within 15 seconds.'));
        }, 15_000);

        const settle = (callback, value) => {
          clearTimeout(timeout);
          callback(value);
        };

        child.once('message', (message) => settle(resolve, message));
        child.once('exit', (code, signal) => {
          settle(
            reject,
            new Error(
              `Standalone server exited before readiness (code=${code}, signal=${signal}).\n${stderr}`,
            ),
          );
        });
      });

      assert.equal(typeof readyMessage, 'object');
      assert.notEqual(readyMessage, null);
      assert.equal(readyMessage.kind, 'ready');
      assert.equal(typeof readyMessage.port, 'number');

      // When: the server renders tracked pages and every excluded page category.
      const trackedPaths = ['/', '/blog/', '/site/submissions/new'];
      const excludedPaths = [
        '/login',
        '/register',
        '/forgot-password',
        '/reset-password',
        '/verify-email',
        '/forbidden',
        '/dashboard',
        '/management/users',
      ];
      const pages = await Promise.all(
        [...trackedPaths, ...excludedPaths].map(async (path) => {
          const authenticated = path === '/dashboard' || path.startsWith('/management/');
          const response = await fetch(`http://127.0.0.1:${readyMessage.port}${path}`, {
            headers: authenticated ? { Cookie: 'heyblog_session=test' } : {},
            redirect: 'manual',
            signal: AbortSignal.timeout(5_000),
          });
          assert.equal(response.status, 200, path);
          return { path, response, html: await response.text() };
        }),
      );

      // Then: both rendering modes authorize their scripts and share one analytics entry.
      for (const { path, response, html } of pages) {
        if (!trackedPaths.includes(path)) {
          assert.doesNotMatch(html, /<script[^>]*src="\/analytics\.js"/u, path);
          assert.doesNotMatch(html, /data-cf-beacon=|G-PH5EGCPHXH/u, path);
          continue;
        }
        const policy =
          response.headers.get('content-security-policy') ??
          html.match(/http-equiv="Content-Security-Policy" content="([^"]+)"/iu)?.[1];
        assert.ok(policy, 'Tracked pages must supply a CSP.');
        for (const [, attributes, source] of html.matchAll(
          /<script\b([^>]*)>([\s\S]*?)<\/script>/gu,
        )) {
          if (
            /\bsrc=/u.test(attributes) ||
            source.trim() === '' ||
            /type="application\//u.test(attributes)
          )
            continue;
          const hash = createHash('sha256').update(source).digest('base64');
          assert.ok(
            policy.includes(`sha256-${hash}`),
            `Inline script is blocked by CSP: sha256-${hash}`,
          );
        }
        assert.equal([...html.matchAll(/<script\b[^>]*src="\/analytics\.js"/gu)].length, 1);
        assert.match(
          html,
          /<script(?=[^>]*\bdefer\b)(?=[^>]*src="\/analytics\.js")[^>]*><\/script>/u,
        );
        assert.doesNotMatch(html, /data-cf-beacon=|G-PH5EGCPHXH/u);
        assert.match(
          policy,
          /connect-src 'self' https:\/\/cloudflareinsights\.com https:\/\/\*\.google-analytics\.com https:\/\/\*\.analytics\.google\.com https:\/\/\*\.googletagmanager\.com/u,
        );
        assert.match(
          policy,
          /script-src 'self' https:\/\/static\.cloudflareinsights\.com\/beacon\.min\.js https:\/\/www\.googletagmanager\.com\/gtag\/js/u,
        );
        assert.doesNotMatch(policy, /unsafe-inline|unsafe-eval/u);
      }

      const loader = await fetch(`http://127.0.0.1:${readyMessage.port}/analytics.js`, {
        signal: AbortSignal.timeout(5_000),
      });
      assert.equal(loader.status, 200);
      assert.match(loader.headers.get('content-type'), /javascript/u);
      assert.match(await loader.text(), /G-PH5EGCPHXH/u);
    } finally {
      if (child.connected) {
        child.send({ kind: 'stop' });
      }
      if (child.exitCode === null) {
        await once(child, 'exit');
      }
      await new Promise((resolve, reject) => {
        apiServer.close((error) => (error ? reject(error) : resolve()));
      });
    }
  });
}
