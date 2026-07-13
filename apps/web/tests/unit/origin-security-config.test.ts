import { readFileSync } from 'node:fs';

import { describe, expect, it } from 'vitest';

const repoRoot = new URL('../../../../', import.meta.url);

const readRepoFile = (path: string): string => readFileSync(new URL(path, repoRoot), 'utf8');

describe('origin security configuration', () => {
  it('trusts the public web origin and local development origins in Astro', () => {
    const config = readRepoFile('apps/web/astro.config.ts');

    expect(config).toContain('allowedDomains');
    expect(config).toContain("protocol: 'https'");
    expect(config).toContain("hostname: 'www.zhblogs.net'");
    expect(config).toContain("hostname: '127.0.0.1'");
    expect(config).toContain("hostname: 'localhost'");
  });

  it('forwards CDN origin requests to Astro as the public web origin', () => {
    const nginx = readRepoFile('infra/nginx/zhblogs.conf');

    expect(nginx).toContain('proxy_set_header Host www.zhblogs.net;');
    expect(nginx).toContain('proxy_set_header X-Forwarded-Host www.zhblogs.net;');
    expect(nginx).toContain('proxy_set_header X-Forwarded-Proto https;');
  });
});
