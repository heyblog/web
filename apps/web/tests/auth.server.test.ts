import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

import { copySetCookie, requestAuthAPI } from '../src/api/auth/auth.server.ts';
import { oauthAuthorizationURL } from '../src/api/auth/oauth.types.ts';
import { readSessionUser, safeNext } from '../src/application/auth/auth.server.ts';

const authEnvironment = {
  WEB_API_BASE_URL: 'http://api.internal:10201',
  API_WEB_TOKEN: 'test-web-service-token-0123456789abcdef',
};

test('auth API forwards only service credentials and browser cookies', async () => {
  // Given
  Object.assign(process.env, authEnvironment);
  const originalFetch = globalThis.fetch;
  let upstreamRequest: Request | undefined;
  globalThis.fetch = async (input, init) => {
    upstreamRequest = new Request(input, init);
    return new Response(JSON.stringify({ user: { id: 'user-id' } }), {
      headers: [
        ['Content-Type', 'application/json'],
        ['Set-Cookie', 'heyblog_access_token=renewed; Path=/; HttpOnly'],
      ],
    });
  };

  try {
    // When
    const response = await requestAuthAPI(
      new Request('https://web.example.test/api/web/auth/me', {
        headers: { Cookie: 'heyblog_access_token=current', 'X-Real-IP': '203.0.113.10' },
      }),
      '/api/web/auth/me',
    );

    // Then
    assert.equal(upstreamRequest?.url, 'http://api.internal:10201/api/web/auth/me');
    assert.equal(upstreamRequest?.headers.get('Cookie'), 'heyblog_access_token=current');
    assert.equal(
      upstreamRequest?.headers.get('X-HeyBlog-Web-Token'),
      authEnvironment.API_WEB_TOKEN,
    );
    assert.equal(upstreamRequest?.headers.get('X-Real-IP'), '203.0.113.10');
    assert.equal(upstreamRequest?.headers.get('X-Forwarded-For'), '203.0.113.10');
    assert.deepEqual(response.headers.getSetCookie(), [
      'heyblog_access_token=renewed; Path=/; HttpOnly',
    ]);
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test('auth API gives the GitHub callback a longer timeout', async () => {
  // Given
  Object.assign(process.env, authEnvironment);
  const originalFetch = globalThis.fetch;
  const originalTimeout = AbortSignal.timeout;
  let requestedTimeout: number | undefined;
  globalThis.fetch = async () => new Response(null, { status: 204 });
  AbortSignal.timeout = (milliseconds) => {
    requestedTimeout = milliseconds;
    return originalTimeout(1_000);
  };

  try {
    // When
    await requestAuthAPI(
      new Request('https://web.example.test/api/web/auth/github/callback'),
      '/api/web/auth/github/callback?code=code&state=state',
    );

    // Then
    assert.equal(requestedTimeout, 55_000);

    await requestAuthAPI(
      new Request('https://web.example.test/api/web/auth/login'),
      '/api/web/auth/login',
    );
    assert.equal(requestedTimeout, 10_000);

    for (const action of ['review', 'review-draft']) {
      await requestAuthAPI(
        new Request(`https://web.example.test/management/site-audits/audit/${action}`),
        `/management/site-audits/audit/${action}`,
        { method: 'POST', body: {} },
      );
      assert.equal(requestedTimeout, 10_000);
    }
  } finally {
    globalThis.fetch = originalFetch;
    AbortSignal.timeout = originalTimeout;
  }
});

test('auth transport forwards management throttling headers without exposing upstream diagnostics', async () => {
  Object.assign(process.env, authEnvironment);
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async () =>
    Response.json(
      { code: 'rate_limited' },
      {
        status: 429,
        headers: {
          'Retry-After': '60',
          'RateLimit-Remaining': '0',
          'X-Internal-Debug': 'private',
        },
      },
    );
  try {
    const response = await requestAuthAPI(
      new Request('https://web.example.test/management/tags/data'),
      '/management/taxonomy/tags',
    );
    assert.equal(response.status, 429);
    assert.equal(response.headers.get('Retry-After'), '60');
    assert.equal(response.headers.get('RateLimit-Remaining'), '0');
    assert.equal(response.headers.get('X-Internal-Debug'), null);
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test('session reader treats an unauthorized API response as signed out', async () => {
  // Given
  Object.assign(process.env, authEnvironment);
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async () => new Response(null, { status: 401 });

  try {
    // When
    const user = await readSessionUser(
      new Request('https://web.example.test/dashboard', {
        headers: { Cookie: 'heyblog_access_token=expired' },
      }),
    );

    // Then
    assert.equal(user, null);
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test('session reader distinguishes service failure from a signed-out session', async () => {
  Object.assign(process.env, authEnvironment);
  const originalFetch = globalThis.fetch;
  try {
    for (const status of [502, 503]) {
      globalThis.fetch = async () => new Response(null, { status });
      await assert.rejects(
        readSessionUser(
          new Request('https://web.example.test/management', {
            headers: { Cookie: 'heyblog_access_token=current' },
          }),
        ),
        { name: 'SessionUnavailableError' },
      );
    }
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test('safeNext accepts only root-relative same-origin paths', () => {
  // Given / When / Then
  assert.equal(safeNext('/dashboard/account'), '/dashboard/account');
  assert.equal(safeNext('//attacker.example/path'), '/dashboard');
  assert.equal(safeNext('https://attacker.example/path'), '/dashboard');
  assert.equal(safeNext('/\\attacker.example/path'), '/dashboard');
  assert.equal(safeNext('/dashboard\n/path'), '/dashboard');
});

test('copySetCookie preserves multiple upstream cookie headers', () => {
  // Given
  const source = new Response(null, {
    headers: [
      ['Set-Cookie', 'one=1; Path=/'],
      ['Set-Cookie', 'two=2; Path=/'],
    ],
  });
  const target = new Headers();

  // When
  copySetCookie(source, target);

  // Then
  assert.deepEqual(target.getSetCookie(), ['one=1; Path=/', 'two=2; Path=/']);
});

test('OAuth start accepts only the GitHub authorization endpoint', () => {
  assert.equal(
    oauthAuthorizationURL('github', 'https://github.com/login/oauth/authorize?state=state-token'),
    'https://github.com/login/oauth/authorize?state=state-token',
  );
  assert.equal(oauthAuthorizationURL('github', 'https://attacker.example/authorize'), null);
});

test('OAuth entry components avoid prefetching authorization flows', async () => {
  const source = await readFile(
    new URL('../src/components/auth/OAuthButton.svelte', import.meta.url),
    'utf8',
  );
  assert.match(source, /type="button"/u);
  assert.match(source, /data-astro-prefetch="false"/u);
});

test('user management routes share the guarded page loader', async () => {
  const source = await readFile(
    new URL('../src/pages/management/users/index.astro', import.meta.url),
    'utf8',
  );

  assert.match(
    source,
    /import\s*\{[^}]*\bloadUsersPage\b[^}]*\}\s*from\s*['"]@\/application\/management\/users-page\.server['"]/u,
  );
  assert.match(source, /await loadUsersPage\(Astro.request, Astro.locals.sessionUser\)/u);
});
