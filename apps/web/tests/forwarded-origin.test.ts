import assert from 'node:assert/strict';
import { IncomingMessage } from 'node:http';
import { Socket } from 'node:net';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

import type { SSRManifest } from 'astro';
import { createRequestFromNodeRequest, getAbortControllerCleanup } from 'astro/app/node';

import { resolveConfig } from '../node_modules/astro/dist/core/config/config.js';
import { FetchState } from '../node_modules/astro/dist/core/fetch/fetch-state.js';
import { forwardProfile } from '../src/api/account-profile/profile.server.ts';
import { pageLocation } from '../src/application/auth/auth.server.ts';

const { astroConfig: config } = await resolveConfig(
  { root: fileURLToPath(new URL('../', import.meta.url)) },
  'build',
);
const publicOrigin = 'https://www.heyblog.net';
const manifest: SSRManifest = {
  adapterName: '@astrojs/node',
  allowedDomains: config.security.allowedDomains,
  base: config.base,
  trailingSlash: config.trailingSlash,
  buildFormat: config.build.format,
  compressHTML: config.compressHTML,
  checkOrigin: config.security.checkOrigin,
  site: config.site,
  rootDir: config.root,
  srcDir: config.srcDir,
  outDir: config.outDir,
  cacheDir: config.cacheDir,
  publicDir: config.publicDir,
  buildClientDir: config.build.client,
  buildServerDir: config.build.server,
  assetsDir: config.build.assets,
  userAssetsBase: '/',
  serverLike: true,
  middlewareMode: 'classic',
  routes: [],
  renderers: [],
  assets: new Set(),
  clientDirectives: new Map(),
  entryModules: {},
  inlinedScripts: new Map(),
  componentMetadata: new Map(),
  key: crypto.subtle.generateKey({ name: 'AES-GCM', length: 256 }, true, ['encrypt', 'decrypt']),
  i18n: undefined,
  csp: undefined,
  image: {},
  shouldInjectCspMetaTags: false,
  actionBodySizeLimit: config.security.actionBodySizeLimit,
  serverIslandBodySizeLimit: config.security.serverIslandBodySizeLimit,
  devToolbar: {
    enabled: false,
    latestAstroVersion: undefined,
    debugInfoOutput: undefined,
    placement: undefined,
  },
  logLevel: 'silent',
};

function renderedRequest(path: string, headers: Readonly<Record<string, string>> = {}): Request {
  const incoming = new IncomingMessage(new Socket());
  incoming.method = 'POST';
  incoming.url = path;
  incoming.headers = {
    host: 'www.heyblog.net',
    'x-forwarded-host': 'www.heyblog.net',
    'x-forwarded-proto': 'https',
    'x-forwarded-port': '443',
    origin: publicOrigin,
    'sec-fetch-site': 'same-origin',
    'content-type': 'application/json',
    ...headers,
  };
  incoming.push('{}');
  incoming.push(null);
  try {
    // Standalone defers forwarded-header handling until the render state is created.
    const request = createRequestFromNodeRequest(incoming, {
      allowedDomains: config.security.allowedDomains,
      port: config.server.port,
    });
    return new FetchState(manifest, request, undefined, { streaming: false }).request;
  } finally {
    getAbortControllerCleanup(incoming)?.();
  }
}

test('same-origin profile requests reach validation behind the production HTTPS proxy', async () => {
  // Given: the public host and HTTPS default port supplied by the trusted proxy.
  const request = renderedRequest('/api/web/account/profile/confirm');
  // When: the profile boundary receives Astro's final render request.
  const response = await forwardProfile(request, 'profile/confirm');
  // Then: the empty input reaches field validation instead of failing origin checks.
  assert.equal(request.url, `${publicOrigin}/api/web/account/profile/confirm`);
  assert.equal(response.status, 422);
  assert.deepEqual(await response.json(), { status: 422, code: 'invalid_request' });
});

test('auth error redirects retain the public origin behind the production HTTPS proxy', () => {
  // Given: a registration request reconstructed through the standalone render chain.
  const request = renderedRequest('/api/web/auth/register');
  // When: an auth form constructs its local error redirect.
  const location = pageLocation(request, '/register?error=password_mismatch');
  // Then: the redirect stays within the form-action self policy.
  assert.equal(location, `${publicOrigin}/register?error=password_mismatch`);
});

const untrustedHeaders: readonly Readonly<Record<string, string>>[] = [
  { origin: 'https://other.test' },
  { 'sec-fetch-site': 'cross-site' },
  { host: 'other.test', 'x-forwarded-host': 'other.test', origin: 'https://other.test' },
];
for (const headers of untrustedHeaders) {
  test(`profile writes reject untrusted request metadata: ${JSON.stringify(headers)}`, async () => {
    // Given: a foreign origin, cross-site fetch, or untrusted forwarded hostname.
    const request = renderedRequest('/api/web/account/profile/confirm', headers);
    // When: the profile boundary checks the reconstructed request.
    const response = await forwardProfile(request, 'profile/confirm');
    // Then: the mutation is rejected before field validation or upstream access.
    assert.equal(response.status, 403);
    assert.deepEqual(await response.json(), { status: 403, code: 'forbidden' });
  });
}
