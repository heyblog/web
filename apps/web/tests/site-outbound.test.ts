import assert from 'node:assert/strict';
import test from 'node:test';

import { handleImpressions } from '../src/api/site-metrics/site-metrics.server.ts';
import { handleSiteOutbound } from '../src/application/site-outbound/site-outbound.server.ts';
import {
  attributedSiteUrl,
  siteOutboundAttributes,
} from '../src/application/site-outbound/site-outbound.shared.ts';

const unsafeScheme = 'javascript';
const unsafeUrl = `${unsafeScheme}:alert(1)`;
const loadConfig = () => ({ apiBaseUrl: 'http://api.test:10201', apiWebToken: 'test-token' });
const navigationHeaders = {
  'Sec-Fetch-Site': 'same-origin',
  'Sec-Fetch-Mode': 'navigate',
  'Sec-Fetch-Dest': 'document',
  'X-Real-IP': '192.0.2.10',
};

test('external attribution preserves address components and uses the production hostname locally', () => {
  // Given
  const input = 'http://blog.example/base?value=1&utm_source=other&utm_medium=old#section';
  // When
  const result = attributedSiteUrl(input);
  // Then
  assert.equal(
    result,
    'http://blog.example/base?value=1&utm_source=www.heyblog.net&utm_medium=referral#section',
  );
  assert.equal(attributedSiteUrl(unsafeUrl), undefined);
  assert.equal(attributedSiteUrl('https://user:password@blog.example'), undefined);
  assert.deepEqual(siteOutboundAttributes({ shortId: 'A1b2C3d4E' }), {
    href: '/site/out/A1b2C3d4E',
    target: '_blank',
    rel: 'noopener',
    referrerpolicy: 'origin',
    'data-astro-prefetch': 'false',
  });
});

const navigationScenarios: readonly {
  name: string;
  method: string;
  headers: Record<string, string>;
  counted: boolean;
}[] = [
  { name: 'manual navigation', method: 'GET', headers: navigationHeaders, counted: true },
  {
    name: 'automatic navigation without user activation',
    method: 'GET',
    headers: navigationHeaders,
    counted: true,
  },
  {
    name: 'plain HTTP source evidence',
    method: 'GET',
    headers: { Referer: 'http://localhost:19101/site' },
    counted: true,
  },
  { name: 'HEAD', method: 'HEAD', headers: navigationHeaders, counted: false },
  {
    name: 'prefetch',
    method: 'GET',
    headers: { ...navigationHeaders, 'Sec-Purpose': 'prefetch' },
    counted: false,
  },
  {
    name: 'prerender',
    method: 'GET',
    headers: { ...navigationHeaders, Purpose: 'prerender' },
    counted: false,
  },
  { name: 'direct navigation', method: 'GET', headers: {}, counted: false },
  {
    name: 'cross-origin navigation',
    method: 'GET',
    headers: { ...navigationHeaders, 'Sec-Fetch-Site': 'cross-site' },
    counted: false,
  },
  {
    name: 'script fetch',
    method: 'GET',
    headers: { ...navigationHeaders, 'Sec-Fetch-Mode': 'cors', 'Sec-Fetch-Dest': 'empty' },
    counted: false,
  },
];
for (const scenario of navigationScenarios) {
  test(`outbound redirects ${scenario.name} with the correct counting decision`, async () => {
    // Given
    let calls = 0;
    const request = new Request('http://localhost:19101/site/out/A1b2C3d4E', {
      method: scenario.method,
      headers: scenario.headers,
    });
    // When
    const response = await handleSiteOutbound(request, 'A1b2C3d4E', {
      loadConfig,
      fetch: async (input, init) => {
        calls++;
        assert.equal(String(input), 'http://api.test:10201/sites/id/A1b2C3d4E/outbound');
        assert.equal(init?.method, 'POST');
        assert.equal(init?.redirect, 'error');
        const body: unknown = JSON.parse(String(init?.body));
        assert.ok(body && typeof body === 'object' && 'eventId' in body);
        assert.equal(typeof body.eventId === 'string', scenario.counted);
        return Response.json({ homepageUrl: 'http://blog.example/base' });
      },
    });
    // Then
    assert.equal(calls, 1);
    assert.equal(response.status, 302);
    assert.equal(response.headers.get('Cache-Control'), 'no-store');
    assert.equal(response.headers.get('Referrer-Policy'), 'origin');
    assert.equal(
      response.headers.get('Location'),
      'http://blog.example/base?utm_source=www.heyblog.net&utm_medium=referral',
    );
  });
}

test('outbound rejects arbitrary destinations and hides upstream diagnostics', async () => {
  // Given
  const request = new Request(
    'https://www.heyblog.net/site/out/A1b2C3d4E?url=https://evil.example',
  );
  // When
  const response = await handleSiteOutbound(request, 'A1b2C3d4E', {
    loadConfig,
    fetch: async () => {
      assert.fail('unexpected upstream request');
    },
  });
  // Then
  assert.equal(response.status, 404);
  for (const upstream of [
    new Response('private SQL', { status: 500 }),
    Response.json({ homepageUrl: unsafeUrl }),
  ]) {
    const failure = await handleSiteOutbound(
      new Request('https://www.heyblog.net/site/out/A1b2C3d4E'),
      'A1b2C3d4E',
      { loadConfig, fetch: async () => upstream },
    );
    assert.equal(failure.status, 503);
    assert.doesNotMatch(await failure.text(), /SQL|javascript/u);
  }
});

test('impression forwarding preserves identity and throttling without exposing diagnostics', async () => {
  // Given
  const event = { eventId: '12345678-1234-4234-9234-123456789012', shortId: 'A1b2C3d4E' };
  const request = new Request('http://localhost:19101/api/site-metrics/impressions', {
    method: 'POST',
    headers: {
      Referer: 'http://localhost:19101/site',
      'Content-Type': 'application/json',
      'X-Real-IP': '192.0.2.10',
    },
    body: JSON.stringify({ events: [event] }),
  });
  // When
  const response = await handleImpressions(request, {
    loadConfig,
    fetch: async (input, init) => {
      assert.equal(String(input), 'http://api.test:10201/site-metrics/impressions');
      assert.deepEqual(JSON.parse(String(init?.body)), { events: [event] });
      const headers = new Headers(init?.headers);
      assert.equal(headers.get('X-HeyBlog-Web-Token'), 'test-token');
      assert.equal(headers.get('X-Real-IP'), '192.0.2.10');
      assert.equal(headers.get('Cookie'), null);
      return new Response('private diagnostic', { status: 429, headers: { 'Retry-After': '12' } });
    },
  });
  // Then
  assert.equal(response.status, 429);
  assert.equal(response.headers.get('Retry-After'), '12');
  assert.equal(response.headers.get('Cache-Control'), 'no-store');
  assert.doesNotMatch(await response.text(), /private/u);
});

for (const scenario of [
  { body: JSON.stringify({ events: [] }), origin: 'https://www.heyblog.net', status: 400 },
  {
    body: JSON.stringify({ events: [{ eventId: 'bad', shortId: 'A1b2C3d4E' }] }),
    origin: 'https://www.heyblog.net',
    status: 400,
  },
  {
    body: JSON.stringify({
      events: [
        { eventId: '12345678-1234-4234-9234-123456789012', shortId: 'A1b2C3d4E', kind: 'CLICK' },
      ],
    }),
    origin: 'https://www.heyblog.net',
    status: 400,
  },
  { body: 'x'.repeat(32_001), origin: 'https://www.heyblog.net', status: 413 },
  { body: '{}', origin: 'https://evil.example', status: 403 },
]) {
  test(`invalid impression request is rejected with ${scenario.status} before forwarding`, async () => {
    // Given
    const request = new Request('https://www.heyblog.net/api/site-metrics/impressions', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Origin: scenario.origin,
        Referer: scenario.origin,
      },
      body: scenario.body,
    });
    // When
    const response = await handleImpressions(request, {
      loadConfig,
      fetch: async () => {
        assert.fail('unexpected forwarding');
      },
    });
    // Then
    assert.equal(response.status, scenario.status);
  });
}
