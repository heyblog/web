import { Hono } from 'hono';

import { resolveWorkerConfig } from './config';
import type {
  CheckRequestBody,
  CheckResponseData,
  RSSFetchRequestBody,
  RSSFetchResponseData,
  WorkerBindings,
} from './types';

const app = new Hono<{ Bindings: WorkerBindings }>();

app.get('/', (context) =>
  context.json({
    ok: true,
    service: 'zhblogs-cloudflare-worker',
    routes: ['/check', '/rss/fetch'],
  }),
);

app.post('/check', async (context) => {
  const config = resolveWorkerConfig(context.env);
  if (!isAuthorized(context.req.header('authorization'), config.workerCallbackSecret)) {
    return context.json(
      { ok: false, error: { code: 'UNAUTHORIZED', message: 'Invalid bearer token' } },
      401,
    );
  }

  const payload = (await safeJson<CheckRequestBody>(context)) ?? null;
  if (!payload || !isHttpUrl(payload.url)) {
    return context.json(
      { ok: false, error: { code: 'INVALID_BODY', message: 'url must be a valid http/https URL' } },
      400,
    );
  }

  const startedAt = Date.now();
  const timeout = normalizeTimeout(payload.timeout_ms, 12000);

  try {
    const result = await probeUrlWithRetry(payload.url, timeout, 2);
    const durationMS = Date.now() - startedAt;

    const data: CheckResponseData = {
      result: result.result,
      status_code: result.statusCode,
      response_time_ms: result.responseTimeMS,
      duration_ms: Math.max(durationMS, result.responseTimeMS),
      final_url: result.finalURL,
      content_verified: result.contentVerified,
      message: result.message,
    };

    return context.json({ ok: true, data });
  } catch (error) {
    return context.json({ ok: true, data: buildCheckFailureData(payload.url, startedAt, error) });
  }
});

app.post('/rss/fetch', async (context) => {
  const config = resolveWorkerConfig(context.env);
  if (!isAuthorized(context.req.header('authorization'), config.workerCallbackSecret)) {
    return context.json(
      { ok: false, error: { code: 'UNAUTHORIZED', message: 'Invalid bearer token' } },
      401,
    );
  }

  const payload = (await safeJson<RSSFetchRequestBody>(context)) ?? null;
  if (!payload || !isHttpUrl(payload.feed_url)) {
    return context.json(
      {
        ok: false,
        error: { code: 'INVALID_BODY', message: 'feed_url must be a valid http/https URL' },
      },
      400,
    );
  }

  const timeout = normalizeTimeout(payload.timeout_ms, 15000);
  try {
    const response = await fetchWithTimeout(payload.feed_url, timeout);
    const body = await response.text();

    if (response.status >= 400) {
      const data: RSSFetchResponseData = {
        result: 'HTTP_ERROR',
        article_count: 0,
        final_url: response.url || payload.feed_url,
        content_type: response.headers.get('content-type') ?? '',
        content: '',
        message: `status=${response.status}`,
      };
      return context.json({ ok: true, data });
    }

    const finalURL = response.url || payload.feed_url;
    const contentType = response.headers.get('content-type') ?? '';
    const articleCount = countFeedItems(body);
    const data: RSSFetchResponseData = {
      result: 'SUCCESS',
      article_count: articleCount,
      final_url: finalURL,
      content_type: contentType,
      content: body,
      message: 'rss fetched',
    };

    return context.json({ ok: true, data });
  } catch (error) {
    return context.json({ ok: true, data: buildRSSFailureData(payload.feed_url, error) });
  }
});

export default app;

type ProbeResult = {
  result: string;
  statusCode: number;
  responseTimeMS: number;
  finalURL: string;
  contentVerified: boolean;
  message: string;
};

async function probeUrlWithRetry(
  targetURL: string,
  timeoutMS: number,
  maxAttempts: number,
): Promise<ProbeResult> {
  let lastError: unknown;

  for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
    try {
      return await probeUrl(targetURL, timeoutMS);
    } catch (error) {
      lastError = error;
      if (!shouldRetryCheck(error, attempt, maxAttempts)) {
        throw error;
      }
    }
  }

  throw lastError instanceof Error ? lastError : new Error('check failed');
}

async function probeUrl(targetURL: string, timeoutMS: number): Promise<ProbeResult> {
  const startedAt = Date.now();
  const response = await fetchWithTimeout(targetURL, timeoutMS);
  const body = await response.text();
  const responseTimeMS = Date.now() - startedAt;
  const finalURL = response.url || targetURL;
  const contentVerified = verifyContent(body);

  if (response.status >= 400) {
    return {
      result: 'HTTP_ERROR',
      statusCode: response.status,
      responseTimeMS,
      finalURL,
      contentVerified,
      message: `status=${response.status}`,
    };
  }

  return {
    result: 'SUCCESS',
    statusCode: response.status,
    responseTimeMS,
    finalURL,
    contentVerified,
    message: contentVerified ? 'ok' : 'content verification failed',
  };
}

function shouldRetryCheck(error: unknown, attempt: number, maxAttempts: number): boolean {
  if (attempt >= maxAttempts) {
    return false;
  }

  const result = classifyError(error instanceof Error ? error.message : 'check failed');
  return result === 'TIMEOUT' || result === 'FAILURE';
}

function isAuthorized(headerValue: string | undefined, expectedToken: string | undefined): boolean {
  if (!expectedToken || !headerValue) {
    return false;
  }

  const normalized = headerValue.trim();
  if (!normalized.toLowerCase().startsWith('bearer ')) {
    return false;
  }

  return normalized.slice(7).trim() === expectedToken;
}

function isHttpUrl(value: string | undefined): boolean {
  if (!value) {
    return false;
  }

  try {
    const parsed = new URL(value);
    return parsed.protocol === 'http:' || parsed.protocol === 'https:';
  } catch {
    return false;
  }
}

async function safeJson<T>(context: { req: { json: () => Promise<unknown> } }): Promise<T | null> {
  try {
    return (await context.req.json()) as T;
  } catch {
    return null;
  }
}

function normalizeTimeout(value: number | undefined, fallback: number): number {
  if (!Number.isFinite(value)) {
    return fallback;
  }

  return Math.min(30000, Math.max(1000, Math.floor(value ?? fallback)));
}

async function fetchWithTimeout(url: string, timeoutMS: number): Promise<Response> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(new Error('timeout')), timeoutMS);

  try {
    try {
      return await fetch(url, {
        headers: {
          'user-agent': 'zhblogs-cloudflare-worker/1.0',
        },
        signal: controller.signal,
      });
    } catch (error) {
      throw new Error(toErrorMessage(error, 'request failed'), { cause: error });
    }
  } finally {
    clearTimeout(timer);
  }
}

function verifyContent(content: string): boolean {
  const normalized = content.toLowerCase();
  return (
    normalized.includes('<html') ||
    normalized.includes('<rss') ||
    normalized.includes('<feed') ||
    normalized.includes('<entry')
  );
}

function countFeedItems(content: string): number {
  const normalized = content.toLowerCase();
  const rssItems = normalized.match(/<item\b/g)?.length ?? 0;
  const atomEntries = normalized.match(/<entry\b/g)?.length ?? 0;
  return Math.max(rssItems, atomEntries);
}

function classifyError(message: string): string {
  const normalized = message.toLowerCase();
  if (normalized.includes('timeout') || normalized.includes('abort')) {
    return 'TIMEOUT';
  }
  if (normalized.includes('tls') || normalized.includes('certificate')) {
    return 'SSL_ERROR';
  }
  if (
    normalized.includes('dns') ||
    normalized.includes('enotfound') ||
    normalized.includes('getaddrinfo') ||
    normalized.includes('nxdomain') ||
    normalized.includes('gai_strerror') ||
    normalized.includes('no such host') ||
    normalized.includes('resolve') ||
    normalized.includes('service not known')
  ) {
    return 'DNS_ERROR';
  }
  if (normalized.includes('status=')) {
    return 'HTTP_ERROR';
  }
  return 'FAILURE';
}

function toErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message.trim()) {
    return error.message.trim();
  }

  if (typeof error === 'string' && error.trim()) {
    return error.trim();
  }

  return fallback;
}

function buildCheckFailureData(
  targetURL: string,
  startedAt: number,
  error: unknown,
): CheckResponseData {
  const durationMS = Date.now() - startedAt;
  const message = toErrorMessage(error, 'check failed');

  return {
    result: classifyError(message),
    status_code: 0,
    response_time_ms: durationMS,
    duration_ms: durationMS,
    final_url: targetURL,
    content_verified: false,
    message,
  };
}

function buildRSSFailureData(feedURL: string, error: unknown): RSSFetchResponseData {
  const message = toErrorMessage(error, 'rss fetch failed');

  return {
    result: classifyError(message),
    article_count: 0,
    final_url: feedURL,
    content_type: '',
    content: '',
    message,
  };
}

export const __test__ = {
  classifyError,
  countFeedItems,
  isHttpUrl,
  isAuthorized,
  verifyContent,
};
