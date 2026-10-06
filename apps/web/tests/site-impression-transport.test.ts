import assert from 'node:assert/strict';
import test from 'node:test';

import {
  beaconImpressions,
  sendImpressions,
} from '../src/api/site-metrics/site-metrics.browser.ts';

const events = [{ eventId: '12345678-1234-4234-9234-123456789012', shortId: 'A1b2C3d4E' }];

test('acknowledged batches preserve event IDs and same-origin request policy', async () => {
  // Given
  let body = '';
  // When
  const retry = await sendImpressions(events, async (input, init) => {
    assert.equal(input, '/api/site-metrics/impressions');
    assert.equal(init?.credentials, 'same-origin');
    assert.equal(init?.cache, 'no-store');
    body = String(init?.body);
    return new Response(null, { status: 204 });
  });
  // Then
  assert.equal(retry, 0);
  assert.deepEqual(JSON.parse(body), { events });
});

test('unacknowledged batches honor throttling and retain retry eligibility', async () => {
  // Given
  const response = new Response(null, { status: 429, headers: { 'Retry-After': '15' } });
  // When
  const retry = await sendImpressions(events, async () => response);
  // Then
  assert.equal(retry, 15_000);
});

test('accepted Beacon does not also issue keepalive', () => {
  // Given
  let calls = 0;
  // When
  beaconImpressions(
    events,
    {
      sendBeacon: () => {
        calls++;
        return true;
      },
    },
    async () => {
      assert.fail('duplicate delivery');
    },
  );
  // Then
  assert.equal(calls, 1);
});

test('rejected Beacon falls back to keepalive with the original identity', async () => {
  // Given
  let resolveBody: (body: string) => void = () => undefined;
  const delivered = new Promise<string>((resolve) => {
    resolveBody = resolve;
  });
  // When
  beaconImpressions(events, { sendBeacon: () => false }, async (_input, init) => {
    assert.equal(init?.keepalive, true);
    assert.ok(init?.body instanceof Blob);
    resolveBody(await init.body.text());
    return new Response(null, { status: 204 });
  });
  // Then
  assert.deepEqual(JSON.parse(await delivered), { events });
});
