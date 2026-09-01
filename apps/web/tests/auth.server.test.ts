import assert from 'node:assert/strict';
import test from 'node:test';

import {
  copySetCookie,
  readSessionUser,
  requestAuthAPI,
  resolveOAuthLocation,
  safeNext,
} from '../src/application/auth/auth.server.ts';

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
      new Request('https://web.example.test/auth/me', {
        headers: { Cookie: 'heyblog_access_token=current' },
      }),
      '/auth/me',
    );

    // Then
    assert.equal(upstreamRequest?.url, 'http://api.internal:10201/auth/me');
    assert.equal(upstreamRequest?.headers.get('Cookie'), 'heyblog_access_token=current');
    assert.equal(
      upstreamRequest?.headers.get('X-HeyBlog-Web-Token'),
      authEnvironment.API_WEB_TOKEN,
    );
    assert.deepEqual(response.headers.getSetCookie(), [
      'heyblog_access_token=renewed; Path=/; HttpOnly',
    ]);
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

test('safeNext accepts only root-relative same-origin paths', () => {
  // Given / When / Then
  assert.equal(safeNext('/dashboard/account'), '/dashboard/account');
  assert.equal(safeNext('//attacker.example/path'), '/dashboard');
  assert.equal(safeNext('https://attacker.example/path'), '/dashboard');
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

test('OAuth start accepts only the GitHub authorization origin', () => {
  const request = new Request('http://127.0.0.1:10101/auth/github/start');

  assert.equal(
    resolveOAuthLocation(
      request,
      'github/start',
      'https://github.com/login/oauth/authorize?state=state-token',
    ),
    'https://github.com/login/oauth/authorize?state=state-token',
  );
  assert.equal(
    resolveOAuthLocation(request, 'github/start', 'https://attacker.example/authorize'),
    null,
  );
});

test('OAuth callback maps the API redirect back to the current Web origin', () => {
  const request = new Request('http://127.0.0.1:10101/auth/github/callback?code=code&state=state');

  assert.equal(
    resolveOAuthLocation(request, 'github/callback', 'http://api.internal:10201/dashboard'),
    'http://127.0.0.1:10101/dashboard',
  );
});
