import assert from 'node:assert/strict';
import test from 'node:test';

import {
  canonicalSiteRedirectPath,
  type SiteProfile,
} from '../src/application/site-profile/site-profile.server.ts';

const profile: SiteProfile = {
  shortId: '38FycC0ow',
  customId: 'wuke',
  name: '吾柯',
  summary: '测试站点',
  host: 'example.com',
  homepageUrl: 'https://example.com',
  accessScope: 'ALL',
  directoryStatus: 'normal',
  joinedAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-02T00:00:00Z',
  topics: [],
  warnings: [],
  feeds: [],
  resources: [],
  technologies: [],
};

test('redirects a successful UUID site lookup to its short ID path', () => {
  assert.equal(
    canonicalSiteRedirectPath('019ded7f-4be3-7168-8953-b64109326083', {
      kind: 'success',
      data: profile,
    }),
    '/site/38FycC0ow',
  );
});

test('does not redirect a site already requested by its canonical short ID', () => {
  assert.equal(canonicalSiteRedirectPath('38FycC0ow', { kind: 'success', data: profile }), null);
});

test('does not redirect unsuccessful site lookups', () => {
  assert.equal(canonicalSiteRedirectPath('invalid', { kind: 'bad-request' }), null);
  assert.equal(canonicalSiteRedirectPath('invalid', { kind: 'not-found' }), null);
  assert.equal(canonicalSiteRedirectPath('invalid', { kind: 'unavailable' }), null);
});
