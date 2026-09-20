import assert from 'node:assert/strict';
import test from 'node:test';

import {
  defaultApiClientScopes,
  parseApiClientCreatePayload,
  parseApiClientUpdatePayload,
  selectedApiClientScopes,
} from '../src/application/api-keys/api-keys.types.ts';

const baseCreatePayload = {
  name: 'example consumer',
  description: 'calls the example API',
  audience: 'EXTERNAL',
  scopes: ['example.call'],
  expires_at: null,
  never_expires: true,
} as const;

test('accepts example.call for an external API client', () => {
  const payload = parseApiClientCreatePayload({
    ...baseCreatePayload,
    scopes: ['example.call'],
  });

  assert.deepEqual(payload?.scopes, ['example.call']);
});

test('defaults both audiences to example and reads every selected scope', () => {
  const data = new FormData();
  data.append('scopes', 'data_import.write');
  data.append('scopes', 'example.call');

  assert.deepEqual(defaultApiClientScopes('EXTERNAL'), ['example.call']);
  assert.deepEqual(defaultApiClientScopes('INTERNAL'), ['example.call']);
  assert.deepEqual(selectedApiClientScopes(data), ['data_import.write', 'example.call']);
});

test('rejects empty scopes and audience-incompatible scopes at the create boundary', () => {
  assert.equal(parseApiClientCreatePayload({ ...baseCreatePayload, scopes: [] }), null);
  assert.equal(
    parseApiClientCreatePayload({
      ...baseCreatePayload,
      scopes: ['data_import.write'],
    }),
    null,
  );
});

test('accepts example.call and rejects empty scopes at the update boundary', () => {
  assert.deepEqual(
    parseApiClientUpdatePayload({
      name: 'example consumer',
      description: '',
      scopes: ['example.call'],
      enabled: true,
    })?.scopes,
    ['example.call'],
  );
  assert.equal(
    parseApiClientUpdatePayload({
      name: 'example consumer',
      description: '',
      scopes: [],
      enabled: true,
    }),
    null,
  );
});

test('rejects unknown and duplicate scopes', () => {
  assert.equal(
    parseApiClientCreatePayload({ ...baseCreatePayload, scopes: ['unknown.call'] }),
    null,
  );
  assert.equal(
    parseApiClientCreatePayload({ ...baseCreatePayload, scopes: ['example.call', 'example.call'] }),
    null,
  );
});

test('allows internal example grants and rejects the retired scope', () => {
  const payload = {
    ...baseCreatePayload,
    audience: 'INTERNAL',
    never_expires: false,
    expires_at: new Date(Date.now() + 86400000).toISOString(),
  };
  assert.deepEqual(parseApiClientCreatePayload(payload)?.scopes, ['example.call']);
  assert.equal(parseApiClientCreatePayload({ ...baseCreatePayload, scopes: ['sites.read'] }), null);
});
