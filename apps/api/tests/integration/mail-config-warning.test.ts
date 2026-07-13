import { afterEach, describe, expect, it, vi } from 'vitest';

import { createApp } from '@/app/api/service/api.service';
import { TEST_CONFIG } from '@tests/config';

describe('mail config warning', () => {
  let app: ReturnType<typeof createApp> | undefined;

  afterEach(async () => {
    vi.restoreAllMocks();
    vi.unstubAllEnvs();
    await app?.close();
    app = undefined;
  });

  it('warns when production mail links still point to a loopback host', async () => {
    app = createApp({
      disableExternalServices: true,
      envOverrides: {
        ...TEST_CONFIG,
        NODE_ENV: 'production',
        WEB_PUBLIC_BASE_URL: 'http://127.0.0.1:9101',
      },
    });

    const warnSpy = vi.spyOn(app.log, 'warn');

    await app.ready();

    expect(warnSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        baseUrl: 'http://127.0.0.1:9101',
        configKey: 'WEB_PUBLIC_BASE_URL',
      }),
      'mail public web base url points to a loopback host',
    );
  });

  it('keeps the mail public web url separate from the API web base url', async () => {
    app = createApp({
      disableExternalServices: true,
      envOverrides: {
        ...TEST_CONFIG,
        NODE_ENV: 'production',
        API_WEB_BASE_URL: 'http://127.0.0.1:9101',
        WEB_PUBLIC_BASE_URL: 'https://www.zhblogs.net',
      },
    });

    await app.ready();

    expect(app.config.API_WEB_BASE_URL).toBe('http://127.0.0.1:9101');
    expect(app.config.WEB_PUBLIC_BASE_URL).toBe('https://www.zhblogs.net');
  });
});
