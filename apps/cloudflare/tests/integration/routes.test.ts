import { afterEach, describe, expect, it, vi } from 'vitest';

import app from '../../src/index';

type CheckRouteResponse = {
  ok: boolean;
  data?: {
    result: string;
    status_code: number;
    response_time_ms: number;
    duration_ms: number;
    final_url: string;
    content_verified: boolean;
    message: string;
  };
  error?: {
    code: string;
    message: string;
  };
};

type RSSRouteResponse = {
  ok: boolean;
  data?: {
    result: string;
    article_count: number;
    final_url: string;
    content_type: string;
    content: string;
    message: string;
  };
  error?: {
    code: string;
    message: string;
  };
};

async function readJson<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

afterEach(() => {
  vi.restoreAllMocks();
  delete process.env.WORKER_CALLBACK_SECRET;
});

describe('cloudflare worker routes', () => {
  it('rejects unauthorized check requests', async () => {
    const response = await app.request('/check', {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
      },
      body: JSON.stringify({ url: 'https://example.com' }),
    });

    const payload = await readJson<CheckRouteResponse>(response);

    expect(response.status).toBe(401);
    expect(payload.ok).toBe(false);
    expect(payload.error?.code).toBe('UNAUTHORIZED');
  });

  it('probes sites successfully with binding token auth', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response('<html><body>ok</body></html>', {
        status: 200,
        headers: {
          'content-type': 'text/html; charset=utf-8',
        },
      }),
    );

    const response = await app.request(
      '/check',
      {
        method: 'POST',
        headers: {
          authorization: 'Bearer secret-token',
          'content-type': 'application/json',
        },
        body: JSON.stringify({ url: 'https://example.com', timeout_ms: 2500 }),
      },
      { WORKER_CALLBACK_SECRET: 'secret-token' },
    );

    const payload = await readJson<CheckRouteResponse>(response);

    expect(fetchSpy).toHaveBeenCalledTimes(1);
    expect(response.status).toBe(200);
    expect(payload.ok).toBe(true);
    expect(payload.data?.result).toBe('SUCCESS');
    expect(payload.data?.status_code).toBe(200);
    expect(payload.data?.content_verified).toBe(true);
    expect(payload.data?.final_url).toBe('https://example.com');
  });

  it('keeps site availability success when content verification fails', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response('plain text response', {
        status: 200,
        headers: {
          'content-type': 'text/plain; charset=utf-8',
        },
      }),
    );

    const response = await app.request(
      '/check',
      {
        method: 'POST',
        headers: {
          authorization: 'Bearer secret-token',
          'content-type': 'application/json',
        },
        body: JSON.stringify({ url: 'https://example.com/plain.txt', timeout_ms: 2500 }),
      },
      { WORKER_CALLBACK_SECRET: 'secret-token' },
    );

    const payload = await readJson<CheckRouteResponse>(response);

    expect(response.status).toBe(200);
    expect(payload.ok).toBe(true);
    expect(payload.data?.result).toBe('SUCCESS');
    expect(payload.data?.content_verified).toBe(false);
    expect(payload.data?.message).toBe('content verification failed');
  });

  it('retries transient check failures once before succeeding', async () => {
    const fetchSpy = vi
      .spyOn(globalThis, 'fetch')
      .mockRejectedValueOnce(new Error('request timeout'))
      .mockResolvedValueOnce(
        new Response('<html><body>ok</body></html>', {
          status: 200,
          headers: {
            'content-type': 'text/html; charset=utf-8',
          },
        }),
      );

    const response = await app.request(
      '/check',
      {
        method: 'POST',
        headers: {
          authorization: 'Bearer secret-token',
          'content-type': 'application/json',
        },
        body: JSON.stringify({ url: 'https://example.com', timeout_ms: 2500 }),
      },
      { WORKER_CALLBACK_SECRET: 'secret-token' },
    );

    const payload = await readJson<CheckRouteResponse>(response);

    expect(fetchSpy).toHaveBeenCalledTimes(2);
    expect(response.status).toBe(200);
    expect(payload.ok).toBe(true);
    expect(payload.data?.result).toBe('SUCCESS');
  });

  it('does not retry dns lookup failures', async () => {
    const fetchSpy = vi
      .spyOn(globalThis, 'fetch')
      .mockRejectedValue(new Error('DNS lookup failed: name or service not known'));

    const response = await app.request(
      '/check',
      {
        method: 'POST',
        headers: {
          authorization: 'Bearer secret-token',
          'content-type': 'application/json',
        },
        body: JSON.stringify({ url: 'https://blog.zezeshe.com', timeout_ms: 2500 }),
      },
      { WORKER_CALLBACK_SECRET: 'secret-token' },
    );

    const payload = await readJson<CheckRouteResponse>(response);

    expect(fetchSpy).toHaveBeenCalledTimes(1);
    expect(response.status).toBe(200);
    expect(payload.ok).toBe(true);
    expect(payload.data?.result).toBe('DNS_ERROR');
  });

  it('falls back to process env token and returns rss payload', async () => {
    process.env.WORKER_CALLBACK_SECRET = 'env-secret';

    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response('<rss><channel><item>a</item><item>b</item></channel></rss>', {
        status: 200,
        headers: {
          'content-type': 'application/rss+xml',
        },
      }),
    );

    const response = await app.request('/rss/fetch', {
      method: 'POST',
      headers: {
        authorization: 'Bearer env-secret',
        'content-type': 'application/json',
      },
      body: JSON.stringify({ feed_url: 'https://example.com/feed.xml' }),
    });

    const payload = await readJson<RSSRouteResponse>(response);

    expect(response.status).toBe(200);
    expect(payload.ok).toBe(true);
    expect(payload.data?.result).toBe('SUCCESS');
    expect(payload.data?.article_count).toBe(2);
    expect(payload.data?.final_url).toBe('https://example.com/feed.xml');
    expect(payload.data?.content_type).toBe('application/rss+xml');
    expect(payload.data?.content).toContain('<item>a</item>');
  });

  it('classifies rss fetch failures', async () => {
    vi.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('request timeout'));

    const response = await app.request(
      '/rss/fetch',
      {
        method: 'POST',
        headers: {
          authorization: 'Bearer secret-token',
          'content-type': 'application/json',
        },
        body: JSON.stringify({ feed_url: 'https://example.com/feed.xml' }),
      },
      { WORKER_CALLBACK_SECRET: 'secret-token' },
    );

    const payload = await readJson<RSSRouteResponse>(response);

    expect(response.status).toBe(200);
    expect(payload.ok).toBe(true);
    expect(payload.data?.result).toBe('TIMEOUT');
    expect(payload.data?.article_count).toBe(0);
    expect(payload.data?.content).toBe('');
  });
});
