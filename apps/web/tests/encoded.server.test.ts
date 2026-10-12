import assert from 'node:assert/strict';
import { once } from 'node:events';
import { createServer } from 'node:http';
import { gunzipSync, gzipSync } from 'node:zlib';
import test from 'node:test';

import { requestEncoded } from '../src/api/transport/encoded.server.ts';
import { handleApiRequest } from '../src/api/transport/endpoint.server.ts';

test('graph proxy preserves compressed bytes, negotiation and origin variance', async () => {
  const data = Buffer.from(JSON.stringify({ nodes: [], edges: [] }));
  const encoded = gzipSync(data);
  const server = createServer((request, response) => {
    assert.equal(request.headers['accept-encoding'], 'gzip');
    response.writeHead(200, {
      'Content-Type': 'application/json',
      'Content-Encoding': 'gzip',
      Vary: 'Accept-Encoding',
    });
    response.end(encoded);
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  try {
    const address = server.address();
    assert.ok(address && typeof address !== 'string');
    const result = await handleApiRequest(
      new Request('https://web.example.test/api/web/public/site-graph', {
        headers: { 'Accept-Encoding': 'gzip', Origin: 'https://client.example.test' },
      }),
      {
        audience: 'public',
        cors: { allowOrigins: ['https://client.example.test'] },
        method: 'GET',
        upstreamPath: '/graph',
        preserveEncoding: true,
        responseHeaders: ['content-type', 'content-encoding', 'vary'],
      },
      {
        loadConfig: () => ({
          apiBaseUrl: `http://127.0.0.1:${address.port}`,
          apiWebToken: 'test-web-token',
        }),
      },
    );
    assert.equal(result.status, 200);
    assert.equal(result.headers.get('Content-Encoding'), 'gzip');
    assert.equal(result.headers.get('Vary'), 'Origin, Accept-Encoding');
    const bytes = Buffer.from(await result.arrayBuffer());
    assert.deepEqual(bytes, encoded);
    assert.deepEqual(gunzipSync(bytes), data);
  } finally {
    server.closeAllConnections();
    await new Promise<void>((resolve, reject) =>
      server.close((error) => (error ? reject(error) : resolve())),
    );
  }
});

test('encoded transport cancels an in-flight body and preserves bodyless statuses', async () => {
  const server = createServer((request, response) => {
    if (request.url === '/empty') {
      response.writeHead(304);
      response.end();
      return;
    }
    response.writeHead(200, { 'Content-Type': 'application/json' });
    response.write('{');
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  try {
    const address = server.address();
    assert.ok(address && typeof address !== 'string');
    const origin = `http://127.0.0.1:${address.port}`;
    const empty = await requestEncoded(
      new URL('/empty', origin),
      new Headers(),
      AbortSignal.timeout(1_000),
    );
    assert.equal(empty.status, 304);
    assert.equal(empty.body, null);
    const controller = new AbortController();
    const result = await requestEncoded(
      new URL('/stream', origin),
      new Headers(),
      controller.signal,
    );
    const body = result.text();
    controller.abort();
    await assert.rejects(body);
  } finally {
    server.closeAllConnections();
    await new Promise<void>((resolve, reject) =>
      server.close((error) => (error ? reject(error) : resolve())),
    );
  }
});
