import assert from 'node:assert/strict';
import { fork } from 'node:child_process';
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
      const [
        publicResponse,
        authResponse,
        submissionResponse,
        dashboardResponse,
        managementResponse,
      ] = await Promise.all([
        fetch(`http://127.0.0.1:${readyMessage.port}/blog/`, {
          signal: AbortSignal.timeout(5_000),
        }),
        fetch(`http://127.0.0.1:${readyMessage.port}/login`, {
          signal: AbortSignal.timeout(5_000),
        }),
        fetch(`http://127.0.0.1:${readyMessage.port}/site/submissions`, {
          signal: AbortSignal.timeout(5_000),
        }),
        fetch(`http://127.0.0.1:${readyMessage.port}/dashboard`, {
          headers: { Cookie: 'heyblog_session=test' },
          signal: AbortSignal.timeout(5_000),
        }),
        fetch(`http://127.0.0.1:${readyMessage.port}/management/users`, {
          headers: { Cookie: 'heyblog_session=test' },
          signal: AbortSignal.timeout(5_000),
        }),
      ]);
      const [publicHtml, authHtml, submissionHtml, dashboardHtml, managementHtml] =
        await Promise.all([
          publicResponse.text(),
          authResponse.text(),
          submissionResponse.text(),
          dashboardResponse.text(),
          managementResponse.text(),
        ]);

      // Then: non-sensitive pages load both analytics providers under the required CSP.
      assert.equal(publicResponse.status, 200);
      assert.equal(authResponse.status, 200);
      assert.equal(submissionResponse.status, 200);
      assert.equal(dashboardResponse.status, 200);
      assert.equal(managementResponse.status, 200);

      for (const trackedHtml of [publicHtml, submissionHtml]) {
        assert.match(trackedHtml, /data-cf-beacon=/u);
        assert.match(trackedHtml, /d76b65b3f55b4b8f96a0ac0ddcd5493e/u);
        assert.match(trackedHtml, /&quot;spa&quot;:false/u);
        assert.match(
          trackedHtml,
          /<script(?=[^>]*\basync\b)(?=[^>]*type="module")(?=[^>]*src="https:\/\/static\.cloudflareinsights\.com\/beacon\.min\.js")[^>]*>/u,
        );
        assert.match(
          trackedHtml,
          /<script(?=[^>]*\basync\b)(?=[^>]*src="https:\/\/www\.googletagmanager\.com\/gtag\/js\?id=G-PH5EGCPHXH")[^>]*>/u,
        );
        assert.match(trackedHtml, /gtag\('config', 'G-PH5EGCPHXH'\)/u);
      }

      const contentSecurityPolicy = submissionResponse.headers.get('content-security-policy');
      assert.notEqual(contentSecurityPolicy, null);
      assert.match(
        contentSecurityPolicy,
        /connect-src 'self' https:\/\/cloudflareinsights\.com https:\/\/\*\.google-analytics\.com https:\/\/\*\.analytics\.google\.com https:\/\/\*\.googletagmanager\.com/u,
      );
      assert.match(
        contentSecurityPolicy,
        /script-src 'self' https:\/\/static\.cloudflareinsights\.com\/beacon\.min\.js https:\/\/www\.googletagmanager\.com\/gtag\/js/u,
      );
      for (const excludedHtml of [authHtml, dashboardHtml, managementHtml]) {
        assert.doesNotMatch(excludedHtml, /data-cf-beacon=/u);
        assert.doesNotMatch(excludedHtml, /G-PH5EGCPHXH/u);
      }
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
