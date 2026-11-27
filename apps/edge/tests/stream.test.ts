import assert from 'node:assert/strict';
import { once } from 'node:events';
import { createServer } from 'node:http';
import { test } from 'node:test';

import { execute } from '../src/execute.ts';
import type { EdgeRequest } from '../src/protocol.ts';

const request: EdgeRequest = {
  url: 'https://example.com/',
  method: 'GET',
  platform: 'CF',
  headers: {},
  maxBytes: 512,
  timeoutMs: 200,
  resolvedIP: '1.1.1.1',
};

test('a maximum size binary response preserves bytes across stream chunks', async () => {
  const payload = Buffer.alloc(4 << 20);
  for (let index = 0; index < payload.length; index++) {
    payload[index] = (index + Math.floor(index / 4096)) & 0xff;
  }
  const result = await execute(
    { ...request, maxBytes: payload.length, timeoutMs: 1000 },
    {
      platform: 'CF',
      token: 'unused-at-execution-boundary',
      node: {},
      fetch: async () =>
        new Response(
          new ReadableStream<Uint8Array>({
            start(controller) {
              controller.enqueue(payload.subarray(0, 3));
              controller.enqueue(payload.subarray(3));
              controller.close();
            },
          }),
        ),
    },
  );
  assert.equal(result.status, 200);
  assert.equal(result.error, undefined);
  assert.ok(Buffer.from(result.body, 'base64').equals(payload));
  assert.equal(result.body.length, Math.ceil(payload.length / 3) * 4);
});

for (const scenario of ['timeout', 'oversized', 'complete'] as const) {
  test(`native response stream releases its reader after ${scenario}`, async (t) => {
    // Given a real HTTP response that stalls, exceeds the limit, or completes.
    const server = createServer((_, response) => {
      response.writeHead(200);
      switch (scenario) {
        case 'timeout':
          response.write('partial');
          break;
        case 'oversized':
          response.write(Buffer.alloc(request.maxBytes + 1));
          break;
        case 'complete':
          response.end('ok');
          break;
      }
    });
    t.after(async () => {
      server.closeAllConnections();
      await new Promise<void>((resolve, reject) => {
        server.close((error) => (error ? reject(error) : resolve()));
      });
    });
    server.listen(0, '127.0.0.1');
    await once(server, 'listening');
    const address = server.address();
    assert.ok(address && typeof address === 'object');
    const responses: Response[] = [];

    // When native Fetch consumes the stream under the executor's real abort signal.
    const result = await execute(request, {
      platform: 'CF',
      token: 'unused-at-execution-boundary',
      node: {},
      fetch: async (_, init) => {
        const response = await fetch(`http://127.0.0.1:${address.port}/`, init);
        responses.push(response);
        return response;
      },
    });

    // Then cleanup releases the reader even when cancelling an errored stream rejects.
    assert.equal(responses.length, 1);
    assert.equal(responses[0]?.body?.locked, false);
    switch (scenario) {
      case 'timeout':
        assert.equal(result.error, 'target_timeout');
        break;
      case 'oversized':
        assert.equal(result.error, 'response_limit');
        break;
      case 'complete':
        assert.equal(result.error, undefined);
        assert.equal(result.status, 200);
        assert.equal(result.body, btoa('ok'));
        break;
    }
  });
}
