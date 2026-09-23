import assert from 'node:assert/strict';
import test from 'node:test';

import { loadRandomSite, loadSiteGoPage } from '../src/application/site-go/site-go.server.ts';
import {
  buildSiteGoHref,
  buildSiteGoLink,
  legacyRandomDestination,
  parseSiteGoQuery,
  siteGoParameterMessage,
} from '../src/application/site-go/site-go.shared.ts';

test('preserves classification filters when requesting another random site', () => {
  assert.equal(
    buildSiteGoHref('技术', '写作'),
    '/site/go?level1=%E6%8A%80%E6%9C%AF&level2=%E5%86%99%E4%BD%9C',
  );
  assert.equal(buildSiteGoHref('', ''), '/site/go');
});

test('preserves old random links and their query when redirecting', () => {
  assert.equal(
    legacyRandomDestination(
      new URL('https://www.heyblog.net/random?level1=%E6%8A%80%E6%9C%AF&level2=%E5%86%99%E4%BD%9C'),
    ),
    '/site/go?level1=%E6%8A%80%E6%9C%AF&level2=%E5%86%99%E4%BD%9C',
  );
});

test('loads each random selection through the private API boundary', async () => {
  let requestURL = '';
  let token = '';
  const result = await loadRandomSite('?level1=%E6%8A%80%E6%9C%AF', undefined, {
    loadConfig: () => ({ apiBaseUrl: 'http://api.internal:10201', apiWebToken: 'test-token' }),
    fetch: async (input, init) => {
      requestURL = String(input);
      token = new Headers(init?.headers).get('X-HeyBlog-Web-Token') ?? '';
      return Response.json({ site: null });
    },
  });

  assert.deepEqual(result, { kind: 'success', data: { site: null } });
  assert.equal(requestURL, 'http://api.internal:10201/sites/random?level1=%E6%8A%80%E6%9C%AF');
  assert.equal(token, 'test-token');
});

test('strictly parses names and separates preview from selection parameters', () => {
  assert.deepEqual(
    parseSiteGoQuery(new URLSearchParams('level1= 技术 &level2=写作&preview=true')),
    {
      kind: 'valid',
      query: { level1: '技术', level2: '写作', preview: true },
    },
  );
  assert.deepEqual(parseSiteGoQuery(new URLSearchParams()), {
    kind: 'valid',
    query: { level1: '', level2: '', preview: false },
  });
  for (const search of [
    'level1=',
    'level1=  ',
    'level1=技术&level2=',
    'level2=写作',
    'level1=技术&level1=生活',
    'preview=true&preview=true',
    'preview=false',
    'preview=',
    'recommend=true',
    'level1=技%00术',
    `level1=${'长'.repeat(101)}`,
  ]) {
    assert.equal(parseSiteGoQuery(new URLSearchParams(search)).kind, 'invalid', search);
  }
  assert.equal(
    buildSiteGoHref('技术', '', true),
    '/site/go?level1=%E6%8A%80%E6%9C%AF&preview=true',
  );
  const link = new URL(buildSiteGoLink('https://www.heyblog.net', '技术', '写作'));
  assert.equal(link.origin, 'https://www.heyblog.net');
  assert.equal(link.searchParams.get('level1'), '技术');
  assert.equal(link.searchParams.get('level2'), '写作');
  assert.equal(link.searchParams.has('preview'), false);
  assert.equal(
    buildSiteGoLink('https://www.heyblog.net', '', '写作'),
    'https://www.heyblog.net/site/go',
  );
});

const loadConfig = () => ({ apiBaseUrl: 'http://api.internal:10201', apiWebToken: 'test-token' });
const classifications = [
  {
    value: 'tech',
    label: '技术',
    normalCount: 1,
    abnormalCount: 0,
    children: [{ value: 'writing', label: '写作', normalCount: 1, abnormalCount: 0 }],
  },
];

test('preview reaches only the dedicated selection endpoint with Chinese names', async () => {
  const urls: URL[] = [];
  const page = await loadSiteGoPage(
    new URL('https://www.heyblog.net/site/go?level1=技术&preview=true'),
    undefined,
    {
      loadConfig,
      fetch: async (input) => {
        const url = new URL(String(input));
        urls.push(url);
        return Response.json(
          url.pathname === '/sites/options' ? { classifications } : { site: null },
        );
      },
    },
  );
  assert.equal(page.status, 200);
  assert.equal(page.preview, true);
  assert.equal(page.level1, '技术');
  assert.deepEqual(urls.map((url) => url.pathname).sort(), ['/sites/options', '/sites/random']);
  assert.equal(
    urls.find((url) => url.pathname === '/sites/random')?.search,
    '?level1=%E6%8A%80%E6%9C%AF',
  );
});

test('invalid page parameters prevent selection and preserve preview recovery', async () => {
  const page = await loadSiteGoPage(
    new URL('https://www.heyblog.net/site/go?level2=写作&preview=true'),
    undefined,
    {
      loadConfig,
      fetch: async (input) => {
        assert.equal(new URL(String(input)).pathname, '/sites/options');
        return Response.json({ classifications });
      },
    },
  );
  assert.equal(page.status, 400);
  assert.equal(page.recoveryHref, '/site/go?preview=true');
  assert.match(page.message, /一级分类/u);
});

test('semantic errors use safe codes and take precedence over metadata outages', async () => {
  const page = await loadSiteGoPage(
    new URL('https://www.heyblog.net/site/go?level1=技术&level2=旅行'),
    undefined,
    {
      loadConfig,
      fetch: async (input) =>
        new URL(String(input)).pathname === '/sites/random'
          ? Response.json(
              { code: 'random_classification_mismatch', detail: 'private diagnostic' },
              { status: 400 },
            )
          : new Response(null, { status: 503 }),
    },
  );
  assert.equal(page.status, 400);
  assert.match(page.message, /不属于/u);
  assert.doesNotMatch(page.message, /private/u);
  assert.equal(page.editorHref, '/site/go?preview=true');
  assert.doesNotMatch(siteGoParameterMessage('private diagnostic'), /private/u);
});

test('optional metadata failure does not discard normal selections; outages remain distinct', async () => {
  const fixture = { homepageUrl: 'https://example.org' };
  for (const preview of [false, true]) {
    const page = await loadSiteGoPage(
      new URL(`https://www.heyblog.net/site/go?level1=技术${preview ? '&preview=true' : ''}`),
      undefined,
      {
        loadConfig,
        fetch: async (input) =>
          new URL(String(input)).pathname === '/sites/options'
            ? new Response(null, { status: 503 })
            : Response.json({ site: fixture }),
      },
    );
    assert.equal(page.status, preview ? 503 : 200);
    assert.deepEqual(page.site, fixture);
    assert.match(page.rerollHref, /level1=/u);
  }
  const page = await loadSiteGoPage(new URL('https://www.heyblog.net/site/go'), undefined, {
    loadConfig,
    fetch: async () => {
      throw new Error('private diagnostic');
    },
  });
  assert.equal(page.status, 503);
  assert.doesNotMatch(page.message, /private/u);
});
