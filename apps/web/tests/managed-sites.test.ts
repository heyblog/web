import assert from 'node:assert/strict';
import test from 'node:test';

import { parseManagedSitePage } from '../src/api/managed-sites/managed-sites.responses.ts';
import { canManageSites, managementNavigation } from '../src/application/management/navigation.ts';
import { managedSitesPath, managedSitesQuery } from '../src/application/management/sites.model.ts';
import { loadSitesPage } from '../src/application/management/sites-page.server.ts';

const actor = {
  id: 'manager',
  username: 'site_manager',
  display_name: '站点管理员',
  email: 'manager@example.test',
  role: 'ADMIN' as const,
  permissions: ['site.manage'] as const,
  active: true,
  email_verified: true,
};
const site = {
  short_id: 'A1b2C3d4E',
  custom_id: 'example-blog',
  name: 'Example Blog',
  url: 'https://example.com/',
  access_scope: 'ALL',
  visibility: 'VISIBLE',
  revision: 3,
  joined_at: '2026-01-02T03:04:05Z',
  updated_at: '2026-02-03T04:05:06Z',
  pending_audit: { id: '019abcde-0000-7000-8000-000000000000', action: 'UPDATE' },
} as const;
const page = {
  items: [site],
  page: 1,
  page_size: 20,
  total_items: 1,
  total_pages: 1,
};

test('managed site boundary accepts complete pages and rejects malformed site state', () => {
  assert.deepEqual(parseManagedSitePage(page), page);
  assert.equal(
    parseManagedSitePage({ ...page, items: [{ ...site, visibility: 'UNKNOWN' }] }),
    null,
  );
  assert.equal(
    parseManagedSitePage({ ...page, items: [{ ...site, url: 'ftp://example.test' }] }),
    null,
  );
  assert.equal(
    parseManagedSitePage({
      ...page,
      items: [{ ...site, pending_audit: { id: '', action: 'UPDATE' } }],
    }),
    null,
  );
  assert.equal(parseManagedSitePage({ ...page, total_pages: 0 }), null);
});

test('site management query keeps only bounded supported filters', () => {
  assert.deepEqual(
    managedSitesQuery(new URLSearchParams('q=%20Blog%20&visibility=HIDDEN&page=2')),
    {
      q: 'Blog',
      visibility: 'HIDDEN',
      page: 2,
    },
  );
  assert.deepEqual(managedSitesQuery(new URLSearchParams('visibility=bad&page=1.5')), {
    q: '',
    visibility: '',
    page: 1,
  });
  assert.equal(managedSitesQuery(new URLSearchParams({ q: '站'.repeat(161) })).q.length, 160);
  assert.equal(
    managedSitesPath({ q: '个人博客', visibility: 'REMOVED', page: 3 }),
    '/management/sites?q=%E4%B8%AA%E4%BA%BA%E5%8D%9A%E5%AE%A2&visibility=REMOVED&page=3',
  );
});

test('site management navigation follows the dedicated permission', () => {
  assert.equal(canManageSites(actor), true);
  assert.equal(canManageSites({ role: 'ADMIN', permissions: ['site_audit.review'] }), false);
  assert.equal(canManageSites({ role: 'SYS_ADMIN', permissions: [] }), true);
  assert.equal(managementNavigation(actor)[0]?.href, '/management/sites');
});

test('site management page forwards normalized filters and enforces authorization', async () => {
  Object.assign(process.env, {
    WEB_API_BASE_URL: 'http://api.internal:10201',
    API_WEB_TOKEN: 'test-web-service-token-0123456789abcdef',
  });
  const previous = globalThis.fetch;
  const calls: string[] = [];
  const request = new Request(
    'https://web.example.test/management/sites?q=%20Blog%20&visibility=VISIBLE&page=2',
    { headers: { Cookie: 'session=test' } },
  );
  try {
    globalThis.fetch = async (input) => {
      const url = String(input);
      calls.push(url);
      return Response.json({ ...page, page: 1 });
    };
    const ready = await loadSitesPage(request, actor);
    assert.equal(ready.kind, 'ready');
    assert.match(
      calls[0] ?? '',
      /\/api\/web\/management\/sites\?page=2&page_size=20&q=Blog&visibility=VISIBLE$/u,
    );
    if (ready.kind === 'ready') assert.equal(ready.query.page, 1);

    const forbidden = await loadSitesPage(request, {
      ...actor,
      permissions: ['site_audit.review'],
    });
    assert.equal(forbidden.kind, 'redirect');
    if (forbidden.kind === 'redirect') assert.equal(forbidden.location, '/forbidden');

    const anonymous = await loadSitesPage(new Request('https://web.example.test/management/sites'));
    assert.equal(anonymous.kind, 'redirect');
  } finally {
    globalThis.fetch = previous;
  }
});

test('site management page treats malformed successful responses as unavailable', async () => {
  Object.assign(process.env, {
    WEB_API_BASE_URL: 'http://api.internal:10201',
    API_WEB_TOKEN: 'test-web-service-token-0123456789abcdef',
  });
  const previous = globalThis.fetch;
  try {
    globalThis.fetch = async () => Response.json({ items: [] });
    const result = await loadSitesPage(
      new Request('https://web.example.test/management/sites', {
        headers: { Cookie: 'session=test' },
      }),
      actor,
    );
    assert.equal(result.kind, 'error');
    if (result.kind === 'error') assert.equal(result.status, 502);
  } finally {
    globalThis.fetch = previous;
  }
});
