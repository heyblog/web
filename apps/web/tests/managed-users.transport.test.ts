import assert from 'node:assert/strict';
import test from 'node:test';

import {
  mutateManagedUser,
  saveAuthorization,
} from '../src/api/managed-users/managed-users.browser.ts';
import { forwardManagedUserMutation } from '../src/api/managed-users/managed-users.proxy.server.ts';
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
  new Request('https://web.example.test/api/web/management/users/target/permissions', {
    method: 'POST',
    body: new URLSearchParams(`updated_at=${encodeURIComponent(managedUser.updated_at)}&${body}`),
    headers: { Cookie: 'session=test', ...(json ? { Accept: 'application/json' } : {}) },
  });

test('boundary parsing rejects malformed successes and strips private account metadata', () => {
  assert.equal(parseUserResponse({ user: { ...managedUser, role: 'OWNER' } }), null);
  assert.equal(parseUserResponse({ user: { ...managedUser, deletion_pending: 'false' } }), null);
  assert.equal(
    parseUserResponse({ user: { ...managedUser, deletion_pending: true } })?.deletion_pending,
    true,
  );
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
    assert.equal(
      upstream?.url,
      'http://api.internal:10201/api/web/management/users/target/permissions',
    );
    assert.equal(upstream?.headers.get('Cookie'), 'session=test');
    assert.equal(upstream?.headers.get('X-HeyBlog-Web-Token'), environment.API_WEB_TOKEN);
    assert.deepEqual(await upstream?.json(), {
      permissions: ['site.manage'],
      updated_at: managedUser.updated_at,
    });
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
  const pageRequest = new Request('https://web.example.test/api/web/management/users?q=editor', {
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
        String(url).endsWith('/api/web/auth/me')
          ? { user: systemActor }
          : { users: [managedUser], total: 1, page: 1, page_size: 20 },
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
    { kind: 'permissions', permissions: [], updated_at: managedUser.updated_at },
    async (input, init) => {
      calls += 1;
      const sent = new Request(new URL(String(input), 'https://web.example.test'), init);
      assert.equal(sent.headers.get('accept'), 'application/json');
      assert.deepEqual([...(await sent.formData())], [['updated_at', managedUser.updated_at]]);
      return Response.json({ user: managedUser });
    },
  );
  assert.equal(result.ok, true);
  assert.equal(calls, 1);
  assert.deepEqual(
    await saveAuthorization(
      'target',
      { kind: 'role', role: 'USER', updated_at: managedUser.updated_at },
      async () => {
        throw new TypeError('offline');
      },
    ),
    { ok: false, status: 0, code: 'network_error' },
  );
  assert.deepEqual(
    await saveAuthorization(
      'target',
      { kind: 'role', role: 'USER', updated_at: managedUser.updated_at },
      async () => Response.json({ user: systemActor }),
    ),
    { ok: false, status: 502, code: 'invalid_response' },
  );
});

test('detail reads the selected user once with a resolved actor', async () => {
  Object.assign(process.env, environment);
  const previous = globalThis.fetch;
  const calls: string[] = [];
  globalThis.fetch = async (input) => {
    calls.push(String(input));
    return Response.json({ user: managedUser });
  };
  try {
    const result = await loadUsersPage(
      new Request('https://web.example.test/management/users/target', {
        headers: { Cookie: 'session=test' },
      }),
      systemActor,
      'target',
    );
    assert.equal(result.kind, 'ready');
    assert.deepEqual(calls, ['http://api.internal:10201/api/web/management/users/target']);
  } finally {
    globalThis.fetch = previous;
  }
});

test('account creation preserves created user when delivery fails and retry sends only mail', async () => {
  const calls: Array<{ path: string; body: unknown }> = [];
  const fetcher: typeof fetch = async (input, init) => {
    const sent = new Request(new URL(String(input), 'https://web.example.test'), init);
    calls.push({ path: new URL(sent.url).pathname, body: await sent.json() });
    return Response.json({ user: managedUser, mail_sent: calls.length > 1 });
  };
  const created = await mutateManagedUser(
    null,
    { operation: 'create', username: 'editor', display_name: '编辑', email: 'editor@example.test' },
    fetcher,
  );
  assert.deepEqual(created, { ok: true, user: managedUser, mailSent: false });
  const resent = await mutateManagedUser(
    managedUser.id,
    { operation: 'password-reset', updated_at: managedUser.updated_at },
    fetcher,
  );
  assert.deepEqual(resent, { ok: true, user: managedUser, mailSent: true });
  assert.equal(calls[0]?.path, '/api/web/management/users');
  assert.deepEqual(calls[1], {
    path: '/api/web/management/users/target/password-reset',
    body: { updated_at: managedUser.updated_at },
  });
});

test('account proxy enforces origin and size then forwards only accepted account fields', async () => {
  Object.assign(process.env, environment);
  const previous = globalThis.fetch;
  const calls: Request[] = [];
  globalThis.fetch = async (input, init) => {
    calls.push(new Request(input, init));
    return Response.json(
      { user: managedUser },
      { headers: { 'Set-Cookie': 'session=renewed; HttpOnly' } },
    );
  };
  const mutation = (body: unknown, origin = 'https://web.example.test') =>
    new Request('https://web.example.test/api/web/management/users/target/profile', {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json',
        Origin: origin,
        'Sec-Fetch-Site': 'same-origin',
        Cookie: 'session=test',
      },
      body: JSON.stringify(body),
    });
  try {
    const body = {
      display_name: '编辑',
      email: 'editor@example.test',
      updated_at: managedUser.updated_at,
    };
    assert.equal(
      (await forwardManagedUserMutation(mutation(body, 'https://other.test'), 'profile', 'target'))
        .status,
      403,
    );
    assert.equal(
      (
        await forwardManagedUserMutation(
          mutation({ ...body, display_name: 'x'.repeat(9000) }),
          'profile',
          'target',
        )
      ).status,
      422,
    );
    assert.equal(calls.length, 0);
    const response = await forwardManagedUserMutation(
      mutation({ ...body, role: 'SYS_ADMIN' }),
      'profile',
      'target',
    );
    assert.equal(response.status, 200);
    assert.deepEqual(await calls[0]?.json(), body);
    assert.equal(calls[0]?.method, 'PATCH');
    assert.deepEqual(response.headers.getSetCookie(), ['session=renewed; HttpOnly']);
    assert.equal(response.headers.get('Cache-Control'), 'private, no-store');
  } finally {
    globalThis.fetch = previous;
  }
});
