import { describe, expect, it } from 'vitest';

import {
  buildPublicWebUrl,
  isLoopbackPublicWebUrl,
} from '@/infrastructure/mail/http/public-web-url.service';

describe('mail public web url service', () => {
  it('builds a public web url without duplicate slashes', () => {
    expect(
      buildPublicWebUrl('https://www.zhblogs.net/', '/verify-email', {
        token: 'verify-token',
      }),
    ).toBe('https://www.zhblogs.net/verify-email?token=verify-token');
  });

  it('encodes query params for nested return paths', () => {
    expect(
      buildPublicWebUrl('https://www.zhblogs.net', '/reset-password', {
        token: 'reset-token',
        next: '/management/site-submissions?tab=pending',
      }),
    ).toBe(
      'https://www.zhblogs.net/reset-password?token=reset-token&next=%2Fmanagement%2Fsite-submissions%3Ftab%3Dpending',
    );
  });

  it('detects loopback public web base urls', () => {
    expect(isLoopbackPublicWebUrl('http://127.0.0.1:9101')).toBe(true);
    expect(isLoopbackPublicWebUrl('http://localhost:9101')).toBe(true);
    expect(isLoopbackPublicWebUrl('https://www.zhblogs.net')).toBe(false);
  });
});
