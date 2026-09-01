import assert from 'node:assert/strict';
import test from 'node:test';

import {
  getLegacyDomainNoticeFromUrl,
  getUrlWithoutLegacySource,
  LEGACY_DOMAIN_NOTICE_DURATION_MS,
  normalizeLegacyDomain,
} from '../src/application/site-notice/site-notice.shared.ts';

const noticeTarget = {
  hostname: 'www.heyblog.net',
  name: 'HeyBlog',
} as const;

test('normalizes supported legacy domain input forms', () => {
  assert.equal(normalizeLegacyDomain(' ZHBLOGS.NET '), 'zhblogs.net');
  assert.equal(normalizeLegacyDomain('https://www.zhblogs.cn/archive'), 'zhblogs.cn');
  assert.equal(normalizeLegacyDomain('www.zhblogs.org'), 'zhblogs.org');
  assert.equal(normalizeLegacyDomain('https://www.zhblogs.ohyee.cc/path'), 'zhblogs.ohyee.cc');
});

test('rejects empty, malformed, and unsupported legacy domains', () => {
  assert.equal(normalizeLegacyDomain(null), null);
  assert.equal(normalizeLegacyDomain(''), null);
  assert.equal(normalizeLegacyDomain('https://example.com'), null);
  assert.equal(normalizeLegacyDomain('not a domain'), null);
});

test('uses a recognized from source before via', () => {
  const notice = getLegacyDomainNoticeFromUrl(
    'https://www.heyblog.net/blog?from=www.zhblogs.net&via=zhblogs.org',
    noticeTarget,
  );

  assert.equal(notice?.sourceDomain, 'zhblogs.net');
  assert.equal(notice?.tone, 'warning');
  assert.equal(notice?.durationMs, LEGACY_DOMAIN_NOTICE_DURATION_MS);
  assert.equal(notice?.targetHostname, noticeTarget.hostname);
});

test('falls back to a recognized via source', () => {
  const notice = getLegacyDomainNoticeFromUrl(
    'https://www.heyblog.net/docs?from=example.com&via=https://www.zhblogs.org/path',
    noticeTarget,
  );

  assert.equal(notice?.sourceDomain, 'zhblogs.org');
  assert.equal(notice?.tone, 'warning');
});

test('uses one migration notice shape for every supported legacy domain', () => {
  const domains = ['zhblogs.net', 'zhblogs.cn', 'zhblogs.org', 'zhblogs.ohyee.cc'] as const;
  const notices = domains.map((domain) =>
    getLegacyDomainNoticeFromUrl(`https://www.heyblog.net/?from=${domain}`, noticeTarget),
  );
  const reference = notices[0];

  assert.ok(reference);
  for (const notice of notices) {
    assert.equal(notice?.eyebrow, reference.eyebrow);
    assert.equal(notice?.title, reference.title);
    assert.equal(notice?.tone, reference.tone);
    assert.equal(notice?.durationMs, reference.durationMs);
  }
});

test('returns no notice when neither source is recognized', () => {
  const notice = getLegacyDomainNoticeFromUrl(
    'https://www.heyblog.net/?from=example.com&via=other.example',
    noticeTarget,
  );

  assert.equal(notice, null);
});

test('removes redirect tracking while preserving the rest of the URL', () => {
  const cleanUrl = getUrlWithoutLegacySource(
    new URL('https://www.heyblog.net/blog?tag=astro&from=zhblogs.cn&via=zhblogs.org#latest'),
  );

  assert.equal(cleanUrl, '/blog?tag=astro#latest');
});
