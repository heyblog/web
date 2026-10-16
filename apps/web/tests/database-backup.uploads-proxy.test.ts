import assert from 'node:assert/strict';
import test from 'node:test';

import { forwardDatabaseBackup } from '../src/api/database-backup/database-backup.proxy.server.ts';
import { backupPartSize } from '../src/api/database-backup/database-backup.uploads.ts';

import { authenticate, configuration, headers, request } from './database-backup.proxy-fixture.ts';

test('upload controls allow only bounded JSON management actions', async () => {
  const id = '019f033c-2111-7000-9000-000000000004';
  const calls: string[] = [];
  const dependencies = {
    configuration,
    authenticate,
    fetcher: async (input: string | URL | Request, init?: RequestInit) => {
      calls.push(`${init?.method} ${String(input)}`);
      return Response.json({});
    },
  };
  for (const [method, action] of [
    ['POST', 'uploads'],
    ['POST', `uploads/${id}/parts/1`],
    ['POST', `uploads/${id}/complete`],
    ['DELETE', `uploads/${id}`],
  ]) {
    const incoming = new Request(
      `https://web.example.test/api/web/management/database-backup/${action}`,
      { method, headers, ...(method === 'POST' ? { body: '{}' } : {}) },
    );
    assert.equal((await forwardDatabaseBackup(incoming, action ?? '', dependencies)).status, 200);
  }
  assert.equal(
    (
      await forwardDatabaseBackup(
        request({ 'Content-Type': 'multipart/form-data; boundary=synthetic' }),
        'inspect',
        dependencies,
      )
    ).status,
    415,
  );
  assert.equal(
    (await forwardDatabaseBackup(request({ 'Content-Length': '32769' }), 'uploads', dependencies))
      .status,
    413,
  );
  assert.equal(
    (await forwardDatabaseBackup(request(), `uploads/${id}/parts/0`, dependencies)).status,
    404,
  );
  assert.equal(
    (
      await forwardDatabaseBackup(
        new Request(`https://web.example.test/api/web/management/database-backup/uploads/${id}`, {
          method: 'PUT',
          headers,
          body: 'backup bytes',
        }),
        `uploads/${id}`,
        dependencies,
      )
    ).status,
    405,
  );
  assert.equal(calls.length, 4);
});

const partAction = 'uploads/019f033c-2111-7000-9000-000000000004/parts/1';
function binaryRequest(
  body: Uint8Array<ArrayBuffer>,
  overrides: Readonly<Record<string, string>> = {},
) {
  return new Request(`https://web.example.test/api/web/management/database-backup/${partAction}`, {
    method: 'PUT',
    headers: { ...headers, 'Content-Type': 'application/octet-stream', ...overrides },
    body,
  });
}

test('binary upload parts stream through the exact PUT route with private authorization', async () => {
  // Given
  const bytes = new Uint8Array(backupPartSize).fill(7);
  let upstreamSize = 0;
  // When
  const response = await forwardDatabaseBackup(binaryRequest(bytes), partAction, {
    configuration,
    authenticate,
    fetcher: async (input, init) => {
      const incoming = new Request(input, init);
      assert.equal(incoming.method, 'PUT');
      assert.equal(incoming.headers.get('Content-Type'), 'application/octet-stream');
      assert.equal(incoming.headers.get('Cookie'), headers.Cookie);
      assert.equal(incoming.headers.get('X-HeyBlog-Web-Token'), configuration.apiWebToken);
      upstreamSize = (await incoming.arrayBuffer()).byteLength;
      return new Response(null, { status: 204 });
    },
  });
  // Then
  assert.equal(upstreamSize, backupPartSize);
  assert.equal(response.status, 204);
  assert.equal(response.body, null);
  assert.equal(response.headers.get('Cache-Control'), 'private, no-store');
  assert.equal(response.headers.getSetCookie().length, 1);
});

test('binary part policy rejects oversized declarations, media types and cross-origin requests before forwarding', async () => {
  // Given
  let calls = 0;
  const dependencies = {
    configuration,
    authenticate: async () => {
      calls += 1;
      return authenticate(binaryRequest(new Uint8Array()), '/api/web/auth/me');
    },
    fetcher: async () => {
      calls += 1;
      return new Response(null, { status: 204 });
    },
  };
  // When / Then
  for (const [overrides, status] of [
    [{ 'Content-Length': String(backupPartSize + 1) }, 413],
    [{ 'Content-Type': 'application/json' }, 415],
    [{ Origin: 'https://attacker.test' }, 403],
  ] as const)
    assert.equal(
      (
        await forwardDatabaseBackup(
          binaryRequest(new Uint8Array([1]), overrides),
          partAction,
          dependencies,
        )
      ).status,
      status,
    );
  assert.equal(calls, 0);
});

test('binary parts cannot evade the streaming size limit by omitting Content-Length', async () => {
  // Given
  const incoming = binaryRequest(new Uint8Array(backupPartSize + 1));
  // When
  const response = await forwardDatabaseBackup(incoming, partAction, {
    configuration,
    authenticate,
    fetcher: async (input, init) => {
      await new Request(input, init).arrayBuffer();
      return new Response(null, { status: 204 });
    },
  });
  // Then
  assert.equal(response.status, 413);
});
