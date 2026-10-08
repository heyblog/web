import assert from 'node:assert/strict';
import test from 'node:test';

import type { requestAuthAPI } from '../src/api/auth/auth.server.ts';
import { forwardDatabaseBackup } from '../src/api/database-backup/database-backup.proxy.server.ts';
import { backupBodyLimit } from '../src/api/database-backup/database-backup.types.ts';

const configuration = { apiBaseUrl: 'https://internal.example.test', apiWebToken: 't'.repeat(32) };
const headers = {
  Cookie: 'heyblog_access_token=synthetic',
  Origin: 'https://web.example.test',
  'Sec-Fetch-Site': 'same-origin',
  'Content-Type': 'multipart/form-data; boundary=synthetic',
};
function request(overrides: Readonly<Record<string, string>> = {}) {
  return new Request('https://web.example.test/management/database-backup/inspect', {
    method: 'POST',
    headers: { ...headers, ...overrides },
    body: '--synthetic\r\n--synthetic--',
  });
}
const authenticate: typeof requestAuthAPI = async () =>
  Response.json(
    { user: { role: 'SYS_ADMIN' } },
    { headers: { 'Set-Cookie': 'heyblog_access_token=renewed; HttpOnly; Path=/' } },
  );

test('cross-origin backup requests are rejected before credentials or file data are forwarded', async () => {
  // Given
  let calls = 0;
  const dependencies = {
    configuration,
    authenticate: async () => {
      calls += 1;
      return Response.json({});
    },
    fetcher: async () => {
      calls += 1;
      return Response.json({});
    },
  };
  // When
  const response = await forwardDatabaseBackup(
    request({ Origin: 'https://attacker.test' }),
    'inspect',
    dependencies,
  );
  // Then
  assert.equal(response.status, 403);
  assert.equal(calls, 0);
});

test('backup forwarding requires SYS_ADMIN rather than delegated management permissions', async () => {
  // Given
  let forwarded = false;
  // When
  const response = await forwardDatabaseBackup(request(), 'inspect', {
    configuration,
    authenticate: async () =>
      Response.json({ user: { role: 'ADMIN', permissions: ['site.manage', 'user.manage'] } }),
    fetcher: async () => {
      forwarded = true;
      return Response.json({});
    },
  });
  // Then
  assert.equal(response.status, 403);
  assert.equal(forwarded, false);
});

test('multipart backup data is streamed unchanged with private authorization and refreshed cookies', async () => {
  // Given
  const incoming = request();
  const expected = await incoming.clone().text();
  // When
  const response = await forwardDatabaseBackup(incoming, 'inspect', {
    configuration,
    authenticate,
    fetcher: async (input, init) => {
      assert.equal(
        String(input),
        'https://internal.example.test/management/database-backup/inspect',
      );
      assert.equal(
        new Headers(init?.headers).get('X-HeyBlog-Web-Token'),
        configuration.apiWebToken,
      );
      assert.equal(new Headers(init?.headers).get('Cookie'), headers.Cookie);
      assert.equal(new Headers(init?.headers).get('Content-Type'), headers['Content-Type']);
      const upstream = new Request(input, init);
      assert.equal(await upstream.text(), expected);
      return Response.json(
        { checked: true },
        { headers: { Location: 'https://unsafe.test', 'Retry-After': '10' } },
      );
    },
  });
  // Then
  assert.deepEqual(await response.json(), { checked: true });
  assert.equal(response.headers.get('Cache-Control'), 'private, no-store');
  assert.equal(response.headers.get('Location'), null);
  assert.equal(response.headers.get('Retry-After'), '10');
  assert.equal(response.headers.getSetCookie().length, 1);
});

test('export streams the file as an attachment without buffering or allowing upstream filenames', async () => {
  // Given
  const incoming = new Request('https://web.example.test/management/database-backup/export', {
    method: 'POST',
    headers,
    body: 'unexpected ignored form',
  });
  const stream = new ReadableStream<Uint8Array>({
    start(controller) {
      controller.enqueue(new TextEncoder().encode('{"synthetic":true}'));
      controller.close();
    },
  });
  // When
  const response = await forwardDatabaseBackup(incoming, 'export', {
    configuration,
    authenticate,
    fetcher: async (_input, init) => {
      assert.equal(init?.body, '{}');
      return new Response(stream, {
        headers: { 'Content-Disposition': 'inline; filename=unsafe.html' },
      });
    },
  });
  // Then
  assert.equal(
    response.headers.get('Content-Disposition'),
    'attachment; filename="heyblog-database.json"',
  );
  assert.equal(response.headers.get('Content-Type'), 'application/json');
  assert.equal(await response.text(), '{"synthetic":true}');
});

test('large declared uploads and unsupported paths are rejected without contacting the API', async () => {
  // Given
  let calls = 0;
  const dependencies = {
    configuration,
    authenticate,
    fetcher: async () => {
      calls += 1;
      return Response.json({});
    },
  };
  // When
  const large = await forwardDatabaseBackup(
    request({ 'Content-Length': String(backupBodyLimit + 1) }),
    'inspect',
    dependencies,
  );
  const unknown = await forwardDatabaseBackup(request(), 'other/internal-path', dependencies);
  // Then
  assert.equal(large.status, 413);
  assert.equal(unknown.status, 404);
  assert.equal(calls, 0);
});

test('restore network errors report an unknown outcome instead of inviting an automatic retry', async () => {
  // Given
  let calls = 0;
  // When
  const response = await forwardDatabaseBackup(request(), 'restore', {
    configuration,
    authenticate,
    fetcher: async () => {
      calls += 1;
      throw new TypeError('synthetic network failure');
    },
  });
  // Then
  assert.equal(response.status, 502);
  assert.deepEqual(await response.json(), {
    status: 502,
    code: 'restore_outcome_unknown',
    detail: '数据操作无法完成',
  });
  assert.equal(calls, 1);
});
