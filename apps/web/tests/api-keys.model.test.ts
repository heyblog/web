import assert from 'node:assert/strict';
import test from 'node:test';

import {
  activeKeyCount,
  applicableScopes,
  defaultExpiry,
  expirationPayload,
  filterClients,
  keyStatus,
} from '../src/application/api-keys/api-keys.model.ts';
import { parseClient, parseCredential } from '../src/application/api-keys/api-keys.responses.ts';
import { parseApiKeyIssuePayload } from '../src/application/api-keys/api-keys.types.ts';

import { credentialClient, credentialKey, credentialNow } from './api-keys.fixture.ts';

test('derives expiration and revocation before disabled status and counts only live keys', () => {
  // Given
  const expired = { ...credentialKey, expires_at: new Date(credentialNow).toISOString() };
  const revoked = { ...credentialKey, revoked_at: new Date(credentialNow).toISOString() };
  // When / Then
  assert.equal(keyStatus(credentialKey, false, credentialNow), 'active');
  assert.equal(keyStatus(credentialKey, true, credentialNow), 'disabled');
  assert.equal(keyStatus(expired, true, credentialNow), 'expired');
  assert.equal(keyStatus(revoked, true, credentialNow), 'revoked');
  assert.equal(
    activeKeyCount({ ...credentialClient, keys: [credentialKey, expired, revoked] }, credentialNow),
    1,
  );
});

test('switching audience preserves example but removes import without granting extra scopes', () => {
  assert.deepEqual(applicableScopes(['data_import.write', 'example.call'], 'EXTERNAL'), [
    'example.call',
  ]);
  assert.deepEqual(applicableScopes(['example.call'], 'INTERNAL'), ['example.call']);
  assert.deepEqual(applicableScopes(['data_import.write'], 'EXTERNAL'), []);
});

test('validates expiration boundaries for both audiences', () => {
  const at = (days: number) => ({
    expires_at: new Date(credentialNow + days * 86_400_000).toISOString(),
    never_expires: false,
  });
  assert.deepEqual(expirationPayload('INTERNAL', at(90), credentialNow), at(90));
  assert.equal(expirationPayload('INTERNAL', at(91), credentialNow), null);
  assert.equal(expirationPayload('EXTERNAL', at(0), credentialNow), null);
  assert.equal(
    expirationPayload('INTERNAL', { expires_at: null, never_expires: true }, credentialNow),
    null,
  );
  assert.deepEqual(
    expirationPayload('EXTERNAL', { expires_at: null, never_expires: true }, credentialNow),
    { expires_at: null, never_expires: true },
  );
  assert.equal(parseApiKeyIssuePayload({ expires_at: 'invalid', never_expires: false }), null);
  assert.equal(
    parseApiKeyIssuePayload({ expires_at: at(1).expires_at, never_expires: true }),
    null,
  );
  assert.equal(
    Date.parse(defaultExpiry('INTERNAL', credentialNow)),
    credentialNow + 90 * 86_400_000,
  );
});

test('filters name and description by type and enabled state', () => {
  const external = {
    ...credentialClient,
    id: 'external',
    name: '合作服务',
    audience: 'EXTERNAL' as const,
    disabled_at: '2026-09-19T10:00:00Z',
  };
  assert.deepEqual(
    filterClients([credentialClient, external], {
      query: '导入',
      audience: 'INTERNAL',
      status: 'enabled',
    }),
    [credentialClient],
  );
  assert.deepEqual(
    filterClients([credentialClient, external], {
      query: '合作',
      audience: 'EXTERNAL',
      status: 'disabled',
    }),
    [external],
  );
});

test('rejects malformed response details instead of trusting partial credential objects', () => {
  assert.deepEqual(parseClient({ client: credentialClient }), credentialClient);
  assert.equal(
    parseClient({ client: { ...credentialClient, keys: [{ id: credentialKey.id }] } }),
    null,
  );
  assert.equal(
    parseCredential({ credential: { token: 'hbk_test', client: { id: 'id' }, key: { id: 'id' } } }),
    null,
  );
});
