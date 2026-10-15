import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { registerHooks, stripTypeScriptTypes } from 'node:module';
import test from 'node:test';

import { compileModule } from 'svelte/compiler';

import { saveProfile } from '../src/api/account-profile/profile.browser.ts';
import { forwardProfile } from '../src/api/account-profile/profile.server.ts';
import { parseSessionResponse } from '../src/api/auth/auth.responses.ts';
import type { SessionUser } from '../src/api/auth/auth.types.ts';
import { profileRedirect } from '../src/application/auth/profile-routing.ts';

const user: SessionUser = {
  id: 'user-id',
  username: 'reader',
  display_name: 'Reader',
  email: 'reader@example.test',
  avatar_url: null,
  role: 'USER',
  permissions: [],
  active: true,
  access_status: 'ACTIVE',
  deletion_pending: false,
  email_verified: true,
  has_password: false,
  oauth_providers: ['GITHUB'],
  auth_version: 1,
  created_at: '2026-10-01T00:00:00Z',
  updated_at: '2026-10-01T00:00:00Z',
  last_login_at: null,
  profile_confirmation_required: true,
  username_changes_remaining: 1,
};

const controllerURL = new URL(
  '../src/application/auth/profile.controller.svelte.ts',
  import.meta.url,
);
const compiled = compileModule(stripTypeScriptTypes(await readFile(controllerURL, 'utf8')), {
  filename: controllerURL.pathname,
  generate: 'client',
}).js.code;
const hooks = registerHooks({
  load(url, context, next) {
    return url === controllerURL.href
      ? { source: compiled, format: 'module', shortCircuit: true }
      : next(url, context);
  },
});
const { createProfileController } =
  await import('../src/application/auth/profile.controller.svelte.ts');
hooks.deregister();

for (const code of ['unauthenticated', 'session_expired', 'invalid_token']) {
  test(`profile confirmation offers login recovery when the API returns ${code}`, async () => {
    // Given: an OAuth profile whose session can no longer authorize a save.
    const original = globalThis.fetch;
    globalThis.fetch = async () => Response.json({ code }, { status: 401 });
    const controller = createProfileController(user);
    try {
      // When: the user confirms the profile.
      const saved = await controller.submit('confirm');
      // Then: the form exposes login recovery without treating the outcome as uncertain.
      assert.equal(saved, false);
      assert.equal(controller.signIn, true);
      assert.equal(controller.error, '登录已过期，请重新登录后查看资料。');
      assert.equal(controller.uncertain, false);
      assert.equal(controller.busy, false);
    } finally {
      globalThis.fetch = original;
    }
  });
}

test('profile confirmation asks for refresh when the request is forbidden', async () => {
  // Given: the Web boundary rejects the profile request.
  const original = globalThis.fetch;
  globalThis.fetch = async () => Response.json({ code: 'forbidden' }, { status: 403 });
  const controller = createProfileController(user);
  try {
    // When: the user confirms the profile.
    const saved = await controller.submit('confirm');
    // Then: recovery describes refreshing the page rather than changing valid input.
    assert.equal(saved, false);
    assert.equal(controller.error, '暂时无法保存，请刷新页面后重试。');
    assert.equal(controller.signIn, false);
    assert.equal(controller.uncertain, false);
  } finally {
    globalThis.fetch = original;
  }
});

test('pending profiles return to the requested workspace after confirmation', () => {
  const url = new URL('https://web.example.test/management/tags?view=paths');
  assert.equal(
    profileRedirect(url, user),
    '/complete-profile?next=%2Fmanagement%2Ftags%3Fview%3Dpaths',
  );
  assert.equal(profileRedirect(url, { ...user, profile_confirmation_required: false }), null);
  assert.equal(profileRedirect(new URL('https://web.example.test/site'), user), null);
  assert.equal(profileRedirect(url, null), '/login?next=%2Fmanagement%2Ftags%3Fview%3Dpaths');
  const claimURL = new URL(
    'https://web.example.test/dashboard/claims?short_id=abc123456&purpose=address',
  );
  const claimNext = encodeURIComponent(claimURL.pathname + claimURL.search);
  assert.equal(profileRedirect(claimURL, null), `/login?next=${claimNext}`);
  assert.equal(profileRedirect(claimURL, user), `/complete-profile?next=${claimNext}`);
});

test('session parser preserves account policy and rejects unusable quota responses', () => {
  assert.deepEqual(parseSessionResponse({ user }), user);
  assert.equal(parseSessionResponse({ user: { ...user, username_changes_remaining: 2 } }), null);
  assert.equal(
    parseSessionResponse({ user: { ...user, profile_confirmation_required: undefined } }),
    null,
  );
});

test('profile proxy enforces same-origin method and field boundaries before forwarding', async () => {
  const original = globalThis.fetch;
  let calls = 0;
  globalThis.fetch = async () => {
    calls++;
    return Response.json({ user });
  };
  try {
    const request = (method: string, origin: string, body: object) =>
      new Request('https://web.example.test/api/web/account/username', {
        method,
        headers: { Origin: origin, 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
    assert.equal(
      (
        await forwardProfile(
          request('POST', 'https://other.test', { username: 'next' }),
          'username',
        )
      ).status,
      403,
    );
    assert.equal(
      (
        await forwardProfile(
          request('PATCH', 'https://web.example.test', { username: 'next' }),
          'username',
        )
      ).status,
      405,
    );
    assert.equal(
      (
        await forwardProfile(
          request('POST', 'https://web.example.test', {
            username: 'next',
            username_changes_remaining: 1,
          }),
          'username',
        )
      ).status,
      422,
    );
    assert.equal(calls, 0);
  } finally {
    globalThis.fetch = original;
  }
});

test('profile proxy preserves renewed cookies and forwards the selected operation once', async () => {
  Object.assign(process.env, {
    WEB_API_BASE_URL: 'http://api.internal:10201',
    API_WEB_TOKEN: 'test-web-service-token-0123456789abcdef',
  });
  const original = globalThis.fetch;
  const calls: Request[] = [];
  globalThis.fetch = async (input, init) => {
    calls.push(new Request(input, init));
    return Response.json(
      { user },
      { headers: { 'Set-Cookie': 'heyblog_access_token=new; HttpOnly' } },
    );
  };
  try {
    const response = await forwardProfile(
      new Request('https://web.example.test/api/web/account/profile/confirm', {
        method: 'POST',
        headers: {
          Origin: 'https://web.example.test',
          'Content-Type': 'application/json',
          Cookie: 'heyblog_access_token=old',
        },
        body: JSON.stringify({ username: 'reader', display_name: 'Reader' }),
      }),
      'profile/confirm',
    );
    assert.equal(response.status, 200);
    assert.equal(calls.length, 1);
    assert.equal(calls[0]?.url, 'http://api.internal:10201/api/web/account/profile/confirm');
    assert.equal(calls[0]?.headers.get('cookie'), 'heyblog_access_token=old');
    assert.deepEqual(response.headers.getSetCookie(), ['heyblog_access_token=new; HttpOnly']);
    assert.equal(response.headers.get('cache-control'), 'private, no-store');
  } finally {
    globalThis.fetch = original;
  }
});

test('profile transport reports ambiguous failures without retrying a mutation', async () => {
  const original = globalThis.fetch;
  let calls = 0;
  globalThis.fetch = async () => {
    calls++;
    throw new TypeError('network disconnected');
  };
  try {
    assert.deepEqual(await saveProfile({ kind: 'username', username: 'next_reader' }), {
      ok: false,
      code: 'request_failed',
      uncertain: true,
    });
    assert.equal(calls, 1);
  } finally {
    globalThis.fetch = original;
  }
});
