import assert from 'node:assert/strict';
import { execFile, fork } from 'node:child_process';
import { once } from 'node:events';
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import { createServer, request } from 'node:http';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';
import { promisify } from 'node:util';
import test from 'node:test';

import { writeStandaloneStartup } from '../src/integrations/build/standalone-startup.ts';

test('the generated production launcher configures the standalone server before requests arrive', async () => {
  // Given: the adapter entry exposes the real Node server and rejects automatic startup.
  const directory = await mkdtemp(join(tmpdir(), 'heyblog-startup-'));
  const entry = new URL('./entry.mjs', pathToFileURL(`${directory}/`));
  await writeFile(
    entry,
    `
    import assert from 'node:assert/strict';
    import { createServer } from 'node:http';
    assert.equal(process.env.ASTRO_NODE_AUTOSTART, 'disabled');
    export function startServer() {
      return { server: { server: createServer() } };
    }
  `,
  );
  try {
    // When
    await writeStandaloneStartup(pathToFileURL(`${directory}/`));
    const { stdout } = await promisify(execFile)(process.execPath, [join(directory, 'start.mjs')], {
      timeout: 2000,
    });
    // Then
    const configuration: unknown = JSON.parse(stdout);
    assert.ok(configuration && typeof configuration === 'object');
    assert.equal(Reflect.get(configuration, 'request_timeout_ms'), 30 * 60 * 1000);
    assert.equal(Reflect.get(configuration, 'headers_timeout_ms'), 60_000);
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
});

test('the production launcher accepts multipart uploads beyond the adapter receive deadline', async () => {
  // Given: a real Node receiver with a scaled default deadline.
  const directory = await mkdtemp(join(tmpdir(), 'heyblog-slow-upload-'));
  const directoryURL = pathToFileURL(`${directory}/`);
  const entrySource = `
    import { createServer } from 'node:http';
    export function startServer() {
      const server = createServer({ requestTimeout: 100, headersTimeout: 100, connectionsCheckingInterval: 10 }, (req, res) => {
        let bytes = 0;
        req.on('data', (chunk) => { bytes += chunk.length; });
        req.on('end', () => res.end(String(bytes)));
        req.on('error', () => res.destroy());
      });
      server.listen(0, '127.0.0.1', () => process.send({ port: server.address().port }));
      return { server: { server } };
    }
  `;
  await writeFile(new URL('entry.mjs', directoryURL), entrySource);
  await writeStandaloneStartup(directoryURL);
  const baseline = createServer(
    { requestTimeout: 100, headersTimeout: 100, connectionsCheckingInterval: 10 },
    (req, res) => {
      req.on('error', () => res.destroy());
      req.resume();
      req.on('end', () => res.end('unexpected success'));
    },
  );
  const child = fork(new URL('start.mjs', directoryURL), { silent: true });
  try {
    const message: unknown = (
      await once(child, 'message', { signal: AbortSignal.timeout(5000) })
    )[0];
    assert.ok(message && typeof message === 'object');
    const port: unknown = Reflect.get(message, 'port');
    assert.equal(typeof port, 'number');
    assert.ok(typeof port === 'number');
    baseline.listen(0, '127.0.0.1');
    await once(baseline, 'listening');
    const address = baseline.address();
    assert.ok(address && typeof address !== 'string');
    // When / Then: the same delayed body fails on the default and succeeds through the launcher.
    assert.equal((await slowMultipart(address.port)).status, 408);
    const completed = await slowMultipart(port);
    assert.equal(completed.status, 200);
    assert.equal(Number(completed.body), completed.bytes);
  } finally {
    if (child.exitCode === null && child.signalCode === null) {
      const exited = once(child, 'exit');
      child.kill('SIGTERM');
      await exited;
    }
    baseline.closeAllConnections();
    if (baseline.listening)
      await new Promise<void>((resolve, reject) =>
        baseline.close((error) => (error ? reject(error) : resolve())),
      );
    await rm(directory, { recursive: true, force: true });
  }
});

async function slowMultipart(
  port: number,
): Promise<{ readonly status: number; readonly body: string; readonly bytes: number }> {
  const body =
    '--test\r\nContent-Disposition: form-data; name="file"; filename="backup.json"\r\n\r\n{}\r\n--test--\r\n';
  return new Promise((resolve, reject) => {
    const upload = request(
      {
        host: '127.0.0.1',
        port,
        method: 'POST',
        path: '/api/web/management/database-backup/inspect',
        headers: {
          'Content-Type': 'multipart/form-data; boundary=test',
          'Content-Length': Buffer.byteLength(body),
        },
        signal: AbortSignal.timeout(2000),
      },
      (response) => {
        clearTimeout(finish);
        let text = '';
        response.setEncoding('utf8');
        response.on('data', (chunk: string) => {
          text += chunk;
        });
        response.on('error', reject);
        response.on('end', () =>
          resolve({ status: response.statusCode ?? 0, body: text, bytes: Buffer.byteLength(body) }),
        );
      },
    );
    upload.on('error', (error) => {
      clearTimeout(finish);
      reject(error);
    });
    upload.flushHeaders();
    upload.write(body.slice(0, 20));
    const finish = setTimeout(() => upload.end(body.slice(20)), 300);
  });
}
