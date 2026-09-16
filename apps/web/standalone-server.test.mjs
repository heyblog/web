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
  test('standalone server starts and serves a static asset', async () => {
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
      // When: the running standalone server receives a request for a bundled static asset.
      const response = await fetch(`http://127.0.0.1:${readyMessage.port}/favicon.ico`, {
        signal: AbortSignal.timeout(5_000),
      });

      // Then: the production server responds successfully instead of crashing during startup.
      assert.equal(response.status, 200);
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
