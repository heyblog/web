import assert from 'node:assert/strict';
import { once } from 'node:events';
import { createServer, type Server } from 'node:http';
import test from 'node:test';

import { requestBackup } from '../src/api/database-backup/database-backup.http.server.ts';

async function listen(server: Server) {
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  assert.ok(address && typeof address !== 'string');
  return `http://127.0.0.1:${address.port}`;
}
async function close(server: Server) {
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) =>
    server.close((error) => (error ? reject(error) : resolve())),
  );
}
test('native backup transport streams upload and preserves status and separate cookies', async () => {
  const server = createServer((req, res) => {
    void (async () => {
      let body = '';
      for await (const chunk of req) body += String(chunk);
      assert.equal(body, 'firstsecond');
      assert.equal(req.headers['x-heyblog-web-token'], 'synthetic');
      res.writeHead(202, { 'Content-Type': 'application/json', 'Set-Cookie': ['a=1', 'b=2'] });
      res.write('{"accepted":');
      res.end('true}');
    })().catch((error) => res.destroy(error instanceof Error ? error : undefined));
  });
  const url = await listen(server);
  try {
    const body = new ReadableStream<Uint8Array>({
      start(controller) {
        for (const text of ['first', 'second']) controller.enqueue(new TextEncoder().encode(text));
        controller.close();
      },
    });
    const init = {
      method: 'POST',
      headers: { 'X-HeyBlog-Web-Token': 'synthetic' },
      body,
      duplex: 'half',
      signal: AbortSignal.timeout(1000),
    };
    const response = await requestBackup(url, init);
    assert.equal(response.status, 202);
    assert.deepEqual(response.headers.getSetCookie(), ['a=1', 'b=2']);
    assert.deepEqual(await response.json(), { accepted: true });
  } finally {
    await close(server);
  }
});
test('native backup transport propagates cancellation before headers and during response streaming', async () => {
  const server = createServer((req, res) => {
    if (req.url === '/stream') {
      res.writeHead(200);
      res.write('first');
    }
  });
  const url = await listen(server);
  try {
    const waiting = new AbortController();
    const promise = requestBackup(url, { signal: waiting.signal });
    waiting.abort();
    await assert.rejects(promise);
    const streaming = new AbortController();
    const response = await requestBackup(`${url}/stream`, { signal: streaming.signal });
    const text = response.text();
    streaming.abort();
    await assert.rejects(text);
  } finally {
    await close(server);
  }
});

test('an upload stream failure rejects and releases its upstream connection', async () => {
  const server = createServer();
  const url = await listen(server);
  try {
    const body = new ReadableStream<Uint8Array>({
      pull(controller) {
        controller.error(new Error('synthetic upload failure'));
      },
    });
    const init = { method: 'POST', body, duplex: 'half' };
    await assert.rejects(requestBackup(url, init), /synthetic upload failure/u);
  } finally {
    await close(server);
  }
});

test('an early upstream rejection cancels the remaining upload stream', async () => {
  const server = createServer((_req, res) => {
    res.writeHead(413);
    res.end('too large');
  });
  const url = await listen(server);
  let cancelled = false;
  try {
    let sent = false;
    const body = new ReadableStream<Uint8Array>({
      pull(controller) {
        if (!sent) {
          sent = true;
          controller.enqueue(new Uint8Array(1024));
        }
      },
      cancel() {
        cancelled = true;
      },
    });
    const init = { method: 'POST', body, duplex: 'half', signal: AbortSignal.timeout(1000) };
    const response = await requestBackup(url, init);
    assert.equal(response.status, 413);
    assert.equal(await response.text(), 'too large');
    await new Promise<void>((resolve) => setImmediate(resolve));
    assert.equal(cancelled, true);
  } finally {
    await close(server);
  }
});
