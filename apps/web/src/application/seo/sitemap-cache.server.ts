import { createHash } from 'node:crypto';

import type { DynamicSitemapKind, SitemapItem } from '../../api/sitemap/sitemap.server.ts';
import { createSitemapDocuments, type SitemapEntry } from '../../shared/sitemap.ts';

interface SitemapDependencies {
  readonly loadStatic: () => Promise<readonly SitemapEntry[]>;
  readonly loadDynamic: (
    kind: DynamicSitemapKind,
    signal: AbortSignal,
  ) => Promise<readonly SitemapItem[]>;
  readonly now?: () => number;
}
interface SitemapDocument {
  readonly body: string;
  readonly etag: string;
}
interface Snapshot {
  readonly expiresAt: number;
  readonly documents: ReadonlyMap<string, SitemapDocument>;
}

export function createSitemapService(dependencies: SitemapDependencies) {
  // This cache contains only public XML; no incoming cookies, credentials or request state are retained.
  let current: Snapshot | undefined;
  let pending: Promise<Snapshot> | undefined;
  const now = dependencies.now ?? Date.now;
  async function generate(): Promise<Snapshot> {
    const signal = AbortSignal.timeout(20_000);
    const operation = Promise.all([
      dependencies.loadStatic(),
      dependencies.loadDynamic('sites', signal),
      dependencies.loadDynamic('announcements', signal),
    ]);
    const [staticEntries, sites, announcements] = await new Promise<Awaited<typeof operation>>(
      (resolve, reject) => {
        const abort = () =>
          reject(
            signal.reason instanceof Error
              ? signal.reason
              : new Error('Sitemap generation timed out'),
          );
        signal.addEventListener('abort', abort, { once: true });
        void operation.then(
          (value) => {
            signal.removeEventListener('abort', abort);
            resolve(value);
          },
          (error: unknown) => {
            signal.removeEventListener('abort', abort);
            reject(error);
          },
        );
      },
    );
    signal.throwIfAborted();
    const siteEntries = sites.map((item) => {
      if (!('shortId' in item)) throw new TypeError('Unexpected site sitemap item');
      return { path: `/site/${item.shortId}` };
    });
    const announcementEntries = announcements.map((item) => {
      if (!('startsAt' in item)) throw new TypeError('Unexpected announcement sitemap item');
      return {
        path: `/announcements/${item.id}`,
        lastmod: new Date(
          Math.max(
            Date.parse(item.startsAt),
            Date.parse(item.publishedAt),
            Date.parse(item.updatedAt),
          ),
        ).toISOString(),
      };
    });
    const documents = new Map<string, SitemapDocument>();
    for (const [path, body] of createSitemapDocuments({
      static: staticEntries,
      sites: siteEntries,
      announcements: announcementEntries,
    })) {
      documents.set(path, { body, etag: `"${createHash('sha256').update(body).digest('hex')}"` });
    }
    signal.throwIfAborted();
    current = { documents, expiresAt: now() + 60_000 };
    return current;
  }
  async function snapshot(): Promise<Snapshot> {
    if (current && current.expiresAt > now()) return current;
    pending ??= generate().finally(() => {
      pending = undefined;
    });
    return pending;
  }
  return async (path: string, request: Request): Promise<Response> => {
    if (
      path !== '/sitemap.xml' &&
      !/^\/sitemap-(static|sites|announcements)-[1-9]\d*\.xml$/.test(path)
    )
      return new Response(null, { status: 404 });
    try {
      const value = await snapshot();
      const document = value.documents.get(path);
      if (!document)
        return new Response(null, { status: 404, headers: { 'Cache-Control': 'no-store' } });
      const headers = {
        'Content-Type': 'application/xml; charset=utf-8',
        ETag: document.etag,
        'Cache-Control': `public, max-age=0, s-maxage=${Math.max(0, Math.floor((value.expiresAt - now()) / 1_000))}, must-revalidate`,
      };
      const matches = request.headers
        .get('If-None-Match')
        ?.split(',')
        .some((tag) => tag.trim().replace(/^W\//, '') === document.etag || tag.trim() === '*');
      return new Response(matches || request.method === 'HEAD' ? null : document.body, {
        status: matches ? 304 : 200,
        headers,
      });
    } catch (error) {
      // HTTP boundary: never expose transport or data diagnostics in a public sitemap response.
      if (!(error instanceof Error)) throw error;
      return new Response('Sitemap 暂时不可用。', {
        status: 503,
        headers: {
          'Cache-Control': 'no-store',
          'Retry-After': '30',
          'Content-Type': 'text/plain; charset=utf-8',
        },
      });
    }
  };
}
