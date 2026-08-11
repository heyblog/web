import assert from 'node:assert/strict';
import test from 'node:test';

import { loadWebServerConfig } from '../src/config.server.ts';

const validToken = 'test-web-service-token-0123456789abcdef';

test('loads and normalizes the Web API configuration', () => {
  assert.deepEqual(
    loadWebServerConfig({
      WEB_API_BASE_URL: 'https://api.example.test/',
      API_WEB_TOKEN: validToken,
    }),
    {
      apiBaseUrl: 'https://api.example.test',
      apiWebToken: validToken,
    },
  );
});

test('rejects unsafe or incomplete Web API configuration without leaking values', () => {
  const unsafeURL = 'https://user:private-value@api.example.test/path';
  assert.throws(
    () =>
      loadWebServerConfig({
        WEB_API_BASE_URL: unsafeURL,
        API_WEB_TOKEN: validToken,
      }),
    (error: Error) => !error.message.includes('private-value'),
  );
  assert.throws(
    () =>
      loadWebServerConfig({
        WEB_API_BASE_URL: 'https://api.example.test',
        API_WEB_TOKEN: 'short',
      }),
    /API_WEB_TOKEN/,
  );
});
