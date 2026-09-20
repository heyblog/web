import assert from 'node:assert/strict';
import test from 'node:test';

import {
  ApiKeyRequestError,
  createApiKeyClient,
} from '../src/application/api-keys/api-keys.browser.ts';

import { credentialClient, credentialKey } from './api-keys.fixture.ts';

test('issues through same-origin endpoint with credentials, no cache and explicit expiry', async () => {
  // Given
  const payload = { expires_at: null, never_expires: true };
  const credential = { client: credentialClient, key: credentialKey, token: 'hbk_test_only' };
  const api = createApiKeyClient(async (url, init) => {
    assert.equal(url, `/management/api-keys/client/${credentialClient.id}/keys`);
    assert.equal(init?.credentials, 'same-origin');
    assert.equal(init?.cache, 'no-store');
    assert.equal(init?.method, 'POST');
    assert.deepEqual(JSON.parse(String(init?.body)), payload);
    assert.ok(init?.signal);
    return Response.json({ credential }, { status: 201 });
  });
  // When / Then
  assert.deepEqual(await api.issue(credentialClient.id, payload), credential);
});

test('maps conflict codes without exposing upstream diagnostics', async () => {
  const api = createApiKeyClient(async () =>
    Response.json(
      { code: 'api_client_has_active_key', detail: 'private diagnostic' },
      { status: 409 },
    ),
  );
  await assert.rejects(
    api.issue('client', { expires_at: null, never_expires: true }),
    (error: unknown) => {
      assert.ok(error instanceof ApiKeyRequestError);
      assert.equal(error.code, 'api_client_has_active_key');
      assert.equal(error.status, 409);
      assert.equal(error.message.includes('private diagnostic'), false);
      return true;
    },
  );
});

test('does not retry mutation after network failure or malformed success response', async () => {
  let requests = 0;
  const api = createApiKeyClient(async () => {
    requests++;
    throw new TypeError('network failed');
  });
  await assert.rejects(api.rotate('client', 0), ApiKeyRequestError);
  assert.equal(requests, 1);
  const malformed = createApiKeyClient(async () => new Response('invalid JSON'));
  await assert.rejects(
    malformed.list(),
    (error: unknown) => error instanceof ApiKeyRequestError && error.code === 'invalid_response',
  );
});

test('handles bodyless revoke and parses complete list records', async () => {
  await createApiKeyClient(async () => new Response(null, { status: 204 })).revoke('key');
  const clients = await createApiKeyClient(async () =>
    Response.json({ clients: [credentialClient] }),
  ).list();
  assert.deepEqual(clients[0]?.keys, [credentialKey]);
});
