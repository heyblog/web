import assert from 'node:assert/strict';
import test from 'node:test';

import { saveAuthorization } from '../src/api/managed-users/managed-users.browser.ts';
import {
  parseUserResponse,
  parseUsersResponse,
} from '../src/api/managed-users/managed-users.responses.ts';
import { submitAuthorization } from '../src/application/management/users-mutation.server.ts';
import { loadUsersPage } from '../src/application/management/users-page.server.ts';

import { managedUser, systemActor } from './fixtures/managed-users.ts';

const environment = {
  WEB_API_BASE_URL: 'http://api.internal:10201',
  API_WEB_TOKEN: 'test-web-service-token-0123456789abcdef',
};
const request = (body: string, json = true) =>
  new Request('https://web.example.test/management/users/target/permissions', {
    method: 'POST',
    body: new URLSearchParams(body),
    headers: { Cookie: 'session=test', ...(json ? { Accept: 'application/json' } : {}) },
  });

test('boundary parsing rejects malformed successes and strips private account metadata', () => {
  assert.equal(parseUserResponse({ user: { ...managedUser, role: 'OWNER' } }), null);
  assert.equal(parseUserResponse({ user: { ...managedUser, permissions: ['unknown'] } }), null);
  assert.equal(parseUsersResponse({ users: [managedUser, { id: 'broken' }] }), null);
  assert.equal(
    parseUserResponse({ user: { ...managedUser, permissions: ['site.manage', 'site.manage'] } }),
    null,
  );
  assert.deepEqual(
    parseUserResponse({ user: { ...managedUser, auth_version: 99, has_password: true } }),
    managedUser,
  );
  assert.deepEqual(parseUsersResponse({ users: [] }), []);
});

test('mutations preserve PATCH payloads, cookies, JSON errors and traditional redirects', async () => {
  Object.assign(process.env, environment);
  const previous = globalThis.fetch;
  let calls = 0;
  let upstream: Request | undefined;
  globalThis.fetch = async (input, init) => {
    calls += 1;
    upstream = new Request(input, init);
    return Response.json(
      { user: managedUser },
      { headers: { 'Set-Cookie': 'session=renewed; HttpOnly' } },
    );
  };
  try {
    const result = await submitAuthorization(request('permissions=site.manage'), {
      id: 'target',
      operation: 'permissions',
    });
    assert.equal(result.status, 200);
    assert.deepEqual(await result.json(), { user: managedUser });
    assert.equal(upstream?.method, 'PATCH');
    assert.equal(upstream?.url, 'http://api.internal:10201/management/users/target/permissions');
    assert.equal(upstream?.headers.get('Cookie'), 'session=test');
    assert.equal(upstream?.headers.get('X-HeyBlog-Web-Token'), environment.API_WEB_TOKEN);
    assert.deepEqual(await upstream?.json(), { permissions: ['site.manage'] });
    assert.equal(result.headers.get('Cache-Control'), 'private, no-store');
    assert.deepEqual(result.headers.getSetCookie(), ['session=renewed; HttpOnly']);
    const invalid = await submitAuthorization(request('permissions=bad'), {
      id: 'target',
      operation: 'permissions',
    });
    assert.equal(invalid.status, 422);
    assert.equal(calls, 1);
    assert.equal(
      (await submitAuthorization(request('role=SYS_ADMIN'), { id: 'target', operation: 'role' }))
        .status,
      422,
    );
    assert.equal(
      (await submitAuthorization(request(''), { id: 'target/path', operation: 'permissions' }))
        .status,
      422,
    );
    const legacy = await submitAuthorization(request('role=ADMIN', false), {
      id: 'target',
      operation: 'role',
    });
    assert.equal(legacy.status, 303);
    assert.equal(
      legacy.headers.get('Location'),
      'https://web.example.test/management/users?status=updated',
    );
    globalThis.fetch = async () =>
      Response.json(
        { code: 'permission_scope_exceeded', detail: 'internal private detail' },
        { status: 403 },
      );
    const error = await submitAuthorization(request(''), {
      id: 'target',
      operation: 'permissions',
    });
    assert.deepEqual(await error.json(), { code: 'permission_scope_exceeded', status: 403 });
    globalThis.fetch = async () => Response.json({ user: { id: 'broken' } });
    assert.equal(
      (await submitAuthorization(request(''), { id: 'target', operation: 'permissions' })).status,
      502,
    );
  } finally {
    globalThis.fetch = previous;
  }
});

test('page loading distinguishes unauthorized from unavailable and validates actor and list', async () => {
  Object.assign(process.env, environment);
  const previous = globalThis.fetch;
  const pageRequest = new Request('https://web.example.test/management/users?q=editor', {
    headers: { Cookie: 'session=test' },
  });
  try {
    globalThis.fetch = async () => Response.json({ code: 'expired' }, { status: 401 });
    const unauthorized = await loadUsersPage(pageRequest);
    assert.equal(unauthorized.kind, 'redirect');
    if (unauthorized.kind === 'redirect') assert.match(unauthorized.location, /^\/login\?next=/);
    globalThis.fetch = async () => Response.json({ code: 'failure' }, { status: 503 });
    assert.equal((await loadUsersPage(pageRequest)).kind, 'error');
    globalThis.fetch = async (url) =>
      Response.json(
        String(url).endsWith('/auth/me') ? { user: systemActor } : { users: [managedUser] },
      );
    const ready = await loadUsersPage(pageRequest);
    assert.equal(ready.kind, 'ready');
    if (ready.kind === 'ready') assert.deepEqual(ready.users, [managedUser]);
    globalThis.fetch = async () => Response.json({ user: { ...managedUser, role: 'USER' } });
    const forbidden = await loadUsersPage(pageRequest);
    if (forbidden.kind === 'redirect') assert.equal(forbidden.location, '/forbidden');
    else assert.fail('ordinary user must be denied');
  } finally {
    globalThis.fetch = previous;
  }
});

test('browser saves use form encoding, single request and safe uncertain-result feedback', async () => {
  let calls = 0;
  const result = await saveAuthorization(
    'target',
    { kind: 'permissions', permissions: [] },
    async (input, init) => {
      calls += 1;
      const sent = new Request(new URL(String(input), 'https://web.example.test'), init);
      assert.equal(sent.headers.get('accept'), 'application/json');
      assert.deepEqual([...(await sent.formData())], []);
      return Response.json({ user: managedUser });
    },
  );
  assert.equal(result.ok, true);
  assert.equal(calls, 1);
  assert.deepEqual(
    await saveAuthorization('target', { kind: 'role', role: 'USER' }, async () => {
      throw new TypeError('offline');
    }),
    { ok: false, status: 0, code: 'network_error' },
  );
  assert.deepEqual(
    await saveAuthorization('target', { kind: 'role', role: 'USER' }, async () =>
      Response.json({ user: systemActor }),
    ),
    { ok: false, status: 502, code: 'invalid_response' },
  );
});
