import assert from 'node:assert/strict';
import test from 'node:test';

import { oauthAuthorizationURL, parseOAuthCompletion } from '../src/api/auth/oauth.types.ts';
import { completeOAuth, startOAuth } from '../src/application/auth/oauth.server.ts';

const configuration = {
  WEB_API_BASE_URL: 'http://api.internal:10201',
  API_WEB_TOKEN: 'test-web-service-token-0123456789abcdef',
};
const completion = {
  provider: 'blogsclub',
  state: 'state',
  intent: 'site-claim',
  mode: 'popup',
  next: '/dashboard/claims/claim',
  outcome: 'success',
  claim_id: 'claim',
};

test('each provider accepts only its exact authorization endpoint', () => {
  // Given / When / Then
  assert.equal(
    oauthAuthorizationURL('blogsclub', 'https://www.blogsclub.org/Oauth/Login?state=a'),
    'https://www.blogsclub.org/Oauth/Login?state=a',
  );
  for (const url of [
    'https://github.com/login/oauth/authorize',
    'https://www.blogsclub.org/other',
    'https://www.blogsclub.org.attacker.test/Oauth/Login',
    'https://name@www.blogsclub.org/Oauth/Login',
  ])
    assert.equal(oauthAuthorizationURL('blogsclub', url), null);
  assert.equal(
    oauthAuthorizationURL('github', 'https://github.com/login/oauth/authorize?state=a'),
    'https://github.com/login/oauth/authorize?state=a',
  );
});

test('popup start forwards state cookies and returns validated JSON without a redirect', async () => {
  // Given
  Object.assign(process.env, configuration);
  const original = globalThis.fetch;
  globalThis.fetch = async () =>
    Response.json(
      { authorization_url: 'https://www.blogsclub.org/Oauth/Login?state=state', state: 'state' },
      { headers: { 'Set-Cookie': 'state=state; Path=/; HttpOnly' } },
    );
  try {
    // When
    const response = await startOAuth(
      new Request('https://web.example.test/api/web/auth/blogsclub/start?mode=popup', {
        headers: { Accept: 'application/json' },
      }),
      'blogsclub',
    );
    // Then
    assert.equal(response.status, 200);
    assert.deepEqual(response.headers.getSetCookie(), ['state=state; Path=/; HttpOnly']);
    assert.equal(response.headers.get('Location'), null);
    const payload: unknown = await response.json();
    assert.ok(typeof payload === 'object' && payload !== null && 'state' in payload);
    assert.equal(payload.state, 'state');
  } finally {
    globalThis.fetch = original;
  }
});

for (const [provider, intent] of [
  ['github', 'login'],
  ['blogsclub', 'site-claim'],
] as const)
  test(`${provider} callback forwards to its internal API and sanitizes an unsafe next path`, async () => {
    // Given
    Object.assign(process.env, configuration);
    const original = globalThis.fetch;
    let upstreamRequest: Request | undefined;
    globalThis.fetch = async (input, init) => {
      upstreamRequest = new Request(input, init);
      return Response.json(
        { ...completion, provider, intent, next: '//attacker.test' },
        {
          headers: [
            ['Set-Cookie', 'access=token; Path=/; HttpOnly'],
            ['Set-Cookie', 'state=; Path=/; Max-Age=0'],
          ],
        },
      );
    };
    try {
      // When
      const response = await completeOAuth(
        new Request(
          `https://web.example.test/auth/${provider}/callback?code=secret-code&state=state`,
          {
            headers: { Cookie: 'oauth-state=state' },
          },
        ),
        provider,
      );
      // Then
      assert.ok(!(response instanceof Response));
      assert.equal(
        upstreamRequest?.url,
        `http://api.internal:10201/api/web/auth/${provider}/callback?code=secret-code&state=state`,
      );
      assert.equal(upstreamRequest?.headers.get('Cookie'), 'oauth-state=state');
      assert.equal(
        upstreamRequest?.headers.get('X-HeyBlog-Web-Token'),
        configuration.API_WEB_TOKEN,
      );
      assert.equal(response.result.provider, provider);
      assert.equal(response.result.next, '/dashboard');
      assert.equal(response.headers.getSetCookie().length, 2);
      assert.equal(response.headers.get('Referrer-Policy'), 'no-referrer');
      assert.equal(JSON.stringify(response.result).includes('secret-code'), false);
    } finally {
      globalThis.fetch = original;
    }
  });

test('redirect mode continues to the pending claim after a partial binding failure', async () => {
  // Given
  Object.assign(process.env, configuration);
  const original = globalThis.fetch;
  globalThis.fetch = async () =>
    Response.json(
      {
        ...completion,
        mode: 'redirect',
        outcome: 'error',
        error_code: 'blogsclub_site_mismatch',
      },
      { headers: { 'Set-Cookie': 'state=; Path=/; Max-Age=0' } },
    );
  try {
    // When
    const response = await completeOAuth(
      new Request('https://web.example.test/auth/blogsclub/callback?state=state'),
      'blogsclub',
    );
    // Then
    assert.ok(response instanceof Response);
    assert.equal(response.status, 303);
    assert.equal(
      response.headers.get('Location'),
      'https://web.example.test/dashboard/claims/claim?error=blogsclub_site_mismatch',
    );
    assert.equal(response.headers.getSetCookie().length, 1);
  } finally {
    globalThis.fetch = original;
  }
});

test('invalid state callback preserves cleanup cookies and never reports success', async () => {
  // Given
  Object.assign(process.env, configuration);
  const original = globalThis.fetch;
  globalThis.fetch = async () =>
    Response.json(
      { code: 'invalid_oauth_state', detail: 'private upstream details' },
      { status: 400, headers: { 'Set-Cookie': 'state=; Path=/; Max-Age=0' } },
    );
  try {
    // When
    const response = await completeOAuth(
      new Request('https://web.example.test/auth/blogsclub/callback?state=state'),
      'blogsclub',
    );
    // Then
    assert.ok(!(response instanceof Response));
    assert.equal(response.result.outcome, 'error');
    assert.equal(response.result.error_code, 'invalid_oauth_state');
    assert.equal(response.headers.getSetCookie().length, 1);
    assert.equal(JSON.stringify(response.result).includes('private'), false);
  } finally {
    globalThis.fetch = original;
  }
});

test('malformed completion cannot enter the popup result protocol', () => {
  // Given / When / Then
  assert.equal(parseOAuthCompletion({ ...completion, provider: 'unknown' }), null);
  assert.equal(parseOAuthCompletion({ ...completion, outcome: 'error' }), null);
  assert.equal(parseOAuthCompletion({ ...completion, intent: 'delete-account' }), null);
});

test('start rejects a provider URL whose state differs from the state cookie protocol', async () => {
  Object.assign(process.env, configuration);
  const original = globalThis.fetch;
  globalThis.fetch = async () =>
    Response.json({
      authorization_url: 'https://www.blogsclub.org/Oauth/Login?state=other',
      state: 'expected',
    });
  try {
    const response = await startOAuth(
      new Request('https://web.example.test/api/web/auth/blogsclub/start?mode=popup'),
      'blogsclub',
    );
    assert.equal(response.status, 502);
    assert.equal(response.headers.get('Location'), null);
  } finally {
    globalThis.fetch = original;
  }
});
