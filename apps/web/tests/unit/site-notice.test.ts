import { describe, expect, it } from 'vitest';

import {
  DEFAULT_LEGACY_DOMAIN_NOTICE_DURATION_MS,
  DEFAULT_ZHBLOGS_CN_EXPIRES_AT,
  getLegacyDomainNoticeFromUrl,
  normalizeLegacyDomain,
} from '@/application/site-notice/site-notice.service';

describe('site notice helpers', () => {
  it('normalizes raw domains and absolute URLs', () => {
    expect(normalizeLegacyDomain(' zhblogs.cn ')).toBe('zhblogs.cn');
    expect(normalizeLegacyDomain('https://zhblogs.org/path')).toBe('zhblogs.org');
    expect(normalizeLegacyDomain('')).toBeNull();
    expect(normalizeLegacyDomain(null)).toBeNull();
  });

  it('creates a warning notice for zhblogs.ohyee.cc', () => {
    const notice = getLegacyDomainNoticeFromUrl('https://www.zhblogs.net/?from=zhblogs.ohyee.cc');

    expect(notice).toMatchObject({
      durationMs: DEFAULT_LEGACY_DOMAIN_NOTICE_DURATION_MS,
      eyebrow: '域名迁移提醒',
      title: '旧入口已停止使用',
      tone: 'warning',
    });
    expect(notice?.message).toContain('zhblogs.ohyee.cc');
  });

  it('creates an expiry-aware notice for zhblogs.cn', () => {
    const notice = getLegacyDomainNoticeFromUrl('https://www.zhblogs.net/site?from=zhblogs.cn');

    expect(notice?.title).toBe('旧主域名即将到期');
    expect(DEFAULT_ZHBLOGS_CN_EXPIRES_AT).toBe('2027-02-22T19:27:13+08:00');
    expect(notice?.durationMs).toBe(DEFAULT_LEGACY_DOMAIN_NOTICE_DURATION_MS);
    expect(notice?.message).toContain('2027年2月22日');
  });

  it('creates an info notice for zhblogs.org', () => {
    const notice = getLegacyDomainNoticeFromUrl(
      'https://www.zhblogs.net/docs?from=https://zhblogs.org/article',
    );

    expect(notice).toMatchObject({
      durationMs: DEFAULT_LEGACY_DOMAIN_NOTICE_DURATION_MS,
      eyebrow: '访问入口提示',
      title: '当前入口仅作兼容保留',
      tone: 'info',
    });
  });

  it('ignores unknown legacy domains', () => {
    expect(getLegacyDomainNoticeFromUrl('https://www.zhblogs.net/?from=example.com')).toBeNull();
  });
});
