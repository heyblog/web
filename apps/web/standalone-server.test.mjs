import assert from 'node:assert/strict';
import { fork } from 'node:child_process';
import { once } from 'node:events';
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
  test('standalone server limits Cloudflare Web Analytics to public pages', async () => {
    // Given: a production build started through the generated standalone entrypoint.
    const child = fork(new URL(import.meta.url), [], {
      env: { ...process.env, HEYBLOG_STANDALONE_SMOKE_CHILD: '1' },
      silent: true,
    });
    let stderr = '';
    child.stderr?.setEncoding('utf8');
    child.stderr?.on('data', (chunk) => {
      stderr += chunk;
    });

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

    try {
      // When: the running standalone server serves public and private application pages.
      const [publicResponse, authResponse, submissionResponse] = await Promise.all([
        fetch(`http://127.0.0.1:${readyMessage.port}/blog/`, {
          signal: AbortSignal.timeout(5_000),
        }),
        fetch(`http://127.0.0.1:${readyMessage.port}/login`, {
          signal: AbortSignal.timeout(5_000),
        }),
        fetch(`http://127.0.0.1:${readyMessage.port}/site/submissions`, {
          signal: AbortSignal.timeout(5_000),
        }),
      ]);
      const [publicHtml, authHtml, submissionHtml] = await Promise.all([
        publicResponse.text(),
        authResponse.text(),
        submissionResponse.text(),
      ]);

      // Then: only the public page loads the configured analytics beacon under the required CSP.
      assert.equal(publicResponse.status, 200);
      assert.equal(authResponse.status, 200);
      assert.equal(submissionResponse.status, 200);
      assert.match(publicHtml, /data-cf-beacon=/u);
      assert.match(publicHtml, /ef560dfa471541919b19544f5a95c7a4/u);
      assert.match(publicHtml, /&quot;spa&quot;:false/u);
      assert.match(publicHtml, /https:\/\/static\.cloudflareinsights\.com\/beacon\.min\.js/u);
      assert.match(publicHtml, /connect-src 'self' https:\/\/cloudflareinsights\.com/u);
      assert.doesNotMatch(authHtml, /data-cf-beacon=/u);
      assert.doesNotMatch(submissionHtml, /data-cf-beacon=/u);
    } finally {
      if (child.connected) {
        child.send({ kind: 'stop' });
      }
      if (child.exitCode === null) {
        await once(child, 'exit');
      }
    }
  });
}
