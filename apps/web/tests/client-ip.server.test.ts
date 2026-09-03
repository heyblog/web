import assert from 'node:assert/strict';
import test from 'node:test';

import { forwardClientAddress } from '../src/application/api/client-ip.server.ts';

test('forwards one normalized client address from the trusted Web request header', () => {
  const upstream = new Headers();

  forwardClientAddress(
    new Request('https://web.example.test/', { headers: { 'X-Real-IP': ' 2001:db8::1 ' } }),
    upstream,
  );

  assert.equal(upstream.get('X-Real-IP'), '2001:db8::1');
  assert.equal(upstream.get('X-Forwarded-For'), '2001:db8::1');
});

test('rejects missing, malformed, and chained client address headers', () => {
  for (const value of ['', 'not-an-ip', '203.0.113.10, 198.51.100.2']) {
    const upstream = new Headers();
    forwardClientAddress(
      new Request('https://web.example.test/', { headers: { 'X-Real-IP': value } }),
      upstream,
    );
    assert.equal(upstream.has('X-Real-IP'), false);
    assert.equal(upstream.has('X-Forwarded-For'), false);
  }
});
