import assert from 'node:assert/strict';
import test from 'node:test';

import {
  acceptsSiteManagementRoute,
  forwardSiteManagement,
} from '../src/api/site-management/proxy.server.ts';
import {
  parseChallenge,
  parseClaim,
  parseFriendLinks,
} from '../src/api/site-management/site-management.types.ts';

const claim = {
  id: 'claim-id',
  short_id: 'abc123456',
  user_id: 'user-id',
  address: 'https://example.test/',
  method: 'DNS_TXT',
  status: 'PENDING',
  created_at: '2026-10-01T00:00:00Z',
};

test('account forwarding permits only purpose-built methods and paths', () => {
  assert.equal(acceptsSiteManagementRoute('sites/abc123456/updates', 'POST'), true);
  assert.equal(acceptsSiteManagementRoute('sites/abc123456/friend-links/def123456', 'PUT'), true);
  assert.equal(
    acceptsSiteManagementRoute('sites/abc123456/friend-links/by-host/blog.example.test', 'DELETE'),
    true,
  );
  assert.equal(
    acceptsSiteManagementRoute('sites/abc123456/friend-links/by-host/blog.example.test', 'PUT'),
    false,
  );
  assert.equal(
    acceptsSiteManagementRoute('sites/abc123456/friend-links/by-host/../users', 'DELETE'),
    false,
  );
  assert.equal(acceptsSiteManagementRoute('site-claims/claim-id/check', 'POST'), true);
  assert.equal(acceptsSiteManagementRoute('../management/users', 'GET'), false);
  assert.equal(acceptsSiteManagementRoute('site-claims/claim-id/review', 'POST'), false);
  assert.equal(acceptsSiteManagementRoute('site-claims/claim-id/review', 'POST', true), true);
  assert.equal(acceptsSiteManagementRoute('site-ownership/abc123456', 'PUT', true), true);
  assert.equal(acceptsSiteManagementRoute('sites/abc123456', 'DELETE'), false);
});
test('mutations reject cross-origin, malformed and oversized requests before forwarding', async () => {
  const request = (headers: HeadersInit, body = '{}') =>
    new Request('https://web.test/api/account/site-claims', { method: 'POST', headers, body });
  assert.equal(
    (
      await forwardSiteManagement(
        request({ 'Sec-Fetch-Site': 'cross-site', 'Content-Type': 'application/json' }),
        'site-claims',
      )
    ).status,
    403,
  );
  assert.equal(
    (
      await forwardSiteManagement(
        request({
          'Sec-Fetch-Site': 'same-origin',
          Origin: 'https://other.test',
          'Content-Type': 'application/json',
        }),
        'site-claims',
      )
    ).status,
    403,
  );
  assert.equal(
    (
      await forwardSiteManagement(
        request({ 'Sec-Fetch-Site': 'same-origin', 'Content-Type': 'text/plain' }),
        'site-claims',
      )
    ).status,
    415,
  );
  assert.equal(
    (
      await forwardSiteManagement(
        request({ 'Sec-Fetch-Site': 'same-origin', 'Content-Type': 'application/json' }, '{'),
        'site-claims',
      )
    ).status,
    400,
  );
  assert.equal(
    (
      await forwardSiteManagement(
        request(
          { 'Sec-Fetch-Site': 'same-origin', 'Content-Type': 'application/json' },
          JSON.stringify({ evidence: 'x'.repeat(65536) }),
        ),
        'site-claims',
      )
    ).status,
    413,
  );
});
test('authenticated forwarding keeps cookies, cancellation, no-store and strips redirect', async () => {
  Object.assign(process.env, {
    WEB_API_BASE_URL: 'http://api.internal:10201',
    API_WEB_TOKEN: 'test-web-service-token-0123456789abcdef',
  });
  const original = globalThis.fetch;
  let upstream: Request | undefined;
  globalThis.fetch = async (input, init) => {
    upstream = new Request(input, init);
    return new Response('{}', {
      headers: {
        'Content-Type': 'application/json',
        Location: 'https://other.test',
        'Set-Cookie': 'session=renewed; HttpOnly',
      },
    });
  };
  try {
    const response = await forwardSiteManagement(
      new Request('https://web.test/api/account/sites?page=2&extra=secret', {
        headers: { Cookie: 'session=current' },
      }),
      'sites',
    );
    assert.equal(upstream?.url, 'http://api.internal:10201/account/sites?page=2');
    assert.equal(upstream?.headers.get('Cookie'), 'session=current');
    assert.equal(
      upstream?.headers.get('X-HeyBlog-Web-Token'),
      'test-web-service-token-0123456789abcdef',
    );
    assert.equal(response.headers.get('Cache-Control'), 'private, no-store');
    assert.equal(response.headers.get('Location'), null);
    assert.equal(response.headers.getSetCookie().length, 1);
  } finally {
    globalThis.fetch = original;
  }
});
test('claim parser accepts all methods and rejects invalid discriminants', () => {
  for (const method of ['DNS_TXT', 'META', 'FILE', 'MANUAL'])
    assert.equal(parseClaim({ ...claim, method }).method, method);
  assert.throws(() => parseClaim({ ...claim, method: 'UNKNOWN' }));
  assert.throws(() => parseClaim({ ...claim, status: 'APPROVED' }));
  assert.throws(() => parseClaim({ ...claim, id: 42 }));
  assert.equal(
    parseChallenge({
      claim,
      token: 'one-time-token',
      instructions: {
        dns_name: '_heyblog.example.test',
        dns_value: 'token',
        meta: '',
        file_url: '',
        file_content: '',
      },
    }).token,
    'one-time-token',
  );
});
test('friend parser rejects malformed success payloads', () => {
  assert.deepEqual(parseFriendLinks({ items: [], pending: [] }), { items: [], pending: [] });
  assert.throws(() =>
    parseFriendLinks({ items: [{ target_short_id: 'id', is_reciprocal: 'true' }], pending: [] }),
  );
  assert.throws(() => parseFriendLinks({ items: [], pending: [{ id: 'id' }] }));
});
