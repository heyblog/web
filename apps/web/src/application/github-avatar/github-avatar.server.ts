import { createHash } from 'node:crypto';

import { isGithubUsername } from './github-avatar.ts';

const responseCacheControl =
  'public, max-age=86400, s-maxage=604800, stale-while-revalidate=86400, stale-if-error=604800';
const validContentTypes = new Set([
  'image/avif',
  'image/gif',
  'image/jpeg',
  'image/png',
  'image/webp',
]);

interface CachedAvatar {
  body: ArrayBuffer;
  contentType: string;
  etag: string;
  upstreamEtag?: string;
  lastModified?: string;
  freshUntil: number;
  staleUntil: number;
}

export interface GithubAvatarDependencies {
  fetch?: typeof fetch;
  now?: () => number;
  freshTtlMs?: number;
  staleTtlMs?: number;
  maxEntries?: number;
  maxBytes?: number;
  timeoutMs?: number;
}

export type GithubAvatarHandler = (
  username: string | undefined,
  request: Request,
) => Promise<Response>;

export function createGithubAvatarHandler(
  dependencies: GithubAvatarDependencies = {},
): GithubAvatarHandler {
  const fetchAvatar = dependencies.fetch ?? fetch;
  const now = dependencies.now ?? Date.now;
  const freshTtlMs = dependencies.freshTtlMs ?? 6 * 60 * 60 * 1_000;
  const staleTtlMs = dependencies.staleTtlMs ?? 7 * 24 * 60 * 60 * 1_000;
  const maxEntries = dependencies.maxEntries ?? 128;
  const maxBytes = dependencies.maxBytes ?? 1024 * 1024;
  const timeoutMs = dependencies.timeoutMs ?? 10_000;
  const cache = new Map<string, CachedAvatar>();
  const pendingLoads = new Map<string, Promise<CachedAvatar>>();

  if (freshTtlMs <= 0 || staleTtlMs <= 0 || maxEntries <= 0 || maxBytes <= 0 || timeoutMs <= 0) {
    throw new Error('GitHub avatar cache limits must be positive.');
  }

  const loadAvatar = (username: string, existing?: CachedAvatar): Promise<CachedAvatar> => {
    const key = username.toLocaleLowerCase('en-US');
    const pending = pendingLoads.get(key);

    if (pending) {
      return pending;
    }

    const load = fetchGithubAvatar({
      username,
      existing,
      fetchAvatar,
      now,
      freshTtlMs,
      staleTtlMs,
      maxBytes,
      timeoutMs,
    }).finally(() => pendingLoads.delete(key));
    pendingLoads.set(key, load);
    return load;
  };

  return async (username, request) => {
    if (!username || !isGithubUsername(username)) {
      return errorResponse(400);
    }

    const key = username.toLocaleLowerCase('en-US');
    const cached = cache.get(key);

    if (cached && cached.freshUntil > now()) {
      touchCacheEntry(cache, key, cached);
      return avatarResponse(cached, request, 'hit');
    }

    try {
      const loaded = await loadAvatar(username, cached);
      touchCacheEntry(cache, key, loaded);
      evictOldestEntries(cache, maxEntries);
      return avatarResponse(loaded, request, cached ? 'revalidated' : 'miss');
    } catch {
      if (cached && cached.staleUntil > now()) {
        touchCacheEntry(cache, key, cached);
        return avatarResponse(cached, request, 'stale');
      }

      return errorResponse(502);
    }
  };
}

interface FetchGithubAvatarOptions {
  username: string;
  existing?: CachedAvatar;
  fetchAvatar: typeof fetch;
  now: () => number;
  freshTtlMs: number;
  staleTtlMs: number;
  maxBytes: number;
  timeoutMs: number;
}

async function fetchGithubAvatar(options: FetchGithubAvatarOptions): Promise<CachedAvatar> {
  const headers = new Headers({
    Accept: 'image/avif,image/webp,image/png,image/jpeg,image/gif',
    'User-Agent': 'HeyBlog-avatar-cache/1.0',
  });

  if (options.existing?.upstreamEtag) {
    headers.set('If-None-Match', options.existing.upstreamEtag);
  }
  if (options.existing?.lastModified) {
    headers.set('If-Modified-Since', options.existing.lastModified);
  }

  const upstreamUrl = new URL(`${encodeURIComponent(options.username)}.png`, 'https://github.com/');
  upstreamUrl.searchParams.set('size', '96');

  const response = await options.fetchAvatar(upstreamUrl, {
    headers,
    redirect: 'follow',
    signal: AbortSignal.timeout(options.timeoutMs),
  });
  const loadedAt = options.now();

  if (response.status === 304 && options.existing) {
    return {
      ...options.existing,
      freshUntil: loadedAt + options.freshTtlMs,
      staleUntil: loadedAt + options.freshTtlMs + options.staleTtlMs,
    };
  }

  if (!response.ok) {
    throw new Error(`GitHub avatar returned ${response.status}.`);
  }

  const contentType = response.headers.get('Content-Type')?.split(';', 1)[0]?.trim().toLowerCase();
  const contentLength = Number(response.headers.get('Content-Length'));

  if (!contentType || !validContentTypes.has(contentType)) {
    throw new Error('GitHub avatar returned an unsupported content type.');
  }
  if (Number.isFinite(contentLength) && contentLength > options.maxBytes) {
    throw new Error('GitHub avatar exceeds the configured size limit.');
  }

  const body = await response.arrayBuffer();

  if (body.byteLength === 0 || body.byteLength > options.maxBytes) {
    throw new Error('GitHub avatar has an invalid size.');
  }

  return {
    body,
    contentType,
    etag: `"${createHash('sha256').update(new Uint8Array(body)).digest('base64url')}"`,
    upstreamEtag: response.headers.get('ETag') ?? undefined,
    lastModified: response.headers.get('Last-Modified') ?? undefined,
    freshUntil: loadedAt + options.freshTtlMs,
    staleUntil: loadedAt + options.freshTtlMs + options.staleTtlMs,
  };
}

function avatarResponse(
  avatar: CachedAvatar,
  request: Request,
  cacheStatus: 'hit' | 'miss' | 'revalidated' | 'stale',
): Response {
  const headers = new Headers({
    'Cache-Control': responseCacheControl,
    'Content-Type': avatar.contentType,
    'Cross-Origin-Resource-Policy': 'same-origin',
    ETag: avatar.etag,
    'X-Content-Type-Options': 'nosniff',
    'X-HeyBlog-Cache': cacheStatus,
  });

  if (avatar.lastModified) {
    headers.set('Last-Modified', avatar.lastModified);
  }
  if (cacheStatus === 'stale') {
    headers.set('Warning', '110 - "Response is stale"');
  }
  if (
    request.headers
      .get('If-None-Match')
      ?.split(',')
      .some((value) => value.trim() === avatar.etag)
  ) {
    return new Response(null, { status: 304, headers });
  }

  headers.set('Content-Length', String(avatar.body.byteLength));
  return new Response(avatar.body, { headers });
}

function errorResponse(status: 400 | 502): Response {
  return new Response(null, {
    status,
    headers: {
      'Cache-Control': 'no-store',
      'Cross-Origin-Resource-Policy': 'same-origin',
      'X-Content-Type-Options': 'nosniff',
    },
  });
}

function touchCacheEntry(
  cache: Map<string, CachedAvatar>,
  key: string,
  avatar: CachedAvatar,
): void {
  cache.delete(key);
  cache.set(key, avatar);
}

function evictOldestEntries(cache: Map<string, CachedAvatar>, maxEntries: number): void {
  while (cache.size > maxEntries) {
    const oldestKey = cache.keys().next().value;

    if (oldestKey === undefined) {
      return;
    }

    cache.delete(oldestKey);
  }
}
