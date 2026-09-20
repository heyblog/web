import type { ApiJsonResult } from '../api/client.server.ts';
import { loadSiteByIdentifier, type SiteProfile } from '../site-profile/site-profile.server.ts';

import { loadSiteIcon, type SiteIconResult } from './site-og.icon.server.ts';
import { type SiteOgContent, siteOgContent, siteOgImagePath } from './site-og.model.ts';

interface SiteOgDependencies {
  readonly load?: (identifier: string, request: Request) => Promise<ApiJsonResult<SiteProfile>>;
  readonly render: (content: SiteOgContent) => Promise<Uint8Array<ArrayBuffer>>;
  readonly loadIcon?: (profile: SiteProfile, request: Request) => Promise<SiteIconResult>;
}

function unavailable(): Response {
  return new Response(null, {
    status: 503,
    headers: { 'Cache-Control': 'no-store', 'Retry-After': '30' },
  });
}

export function createSiteOgHandler(dependencies: SiteOgDependencies) {
  return async (identifier: string, request: Request): Promise<Response> => {
    const result = await (dependencies.load ?? loadSiteByIdentifier)(identifier, request);
    switch (result.kind) {
      case 'bad-request':
      case 'not-found':
        return new Response(null, { status: 404, headers: { 'Cache-Control': 'no-store' } });
      case 'unavailable':
        return unavailable();
      case 'success': {
        if (identifier !== result.data.shortId) {
          return new Response(null, {
            status: 308,
            headers: {
              Location: siteOgImagePath(result.data),
              'Cache-Control': 'no-store',
            },
          });
        }
        try {
          const icon = await (dependencies.loadIcon ?? loadSiteIcon)(result.data, request);
          const bytes = await dependencies.render({
            ...siteOgContent(result.data),
            iconDataUrl: icon.kind === 'ready' ? icon.dataUrl : undefined,
          });
          return new Response(bytes, {
            headers: {
              'Content-Type': 'image/png',
              // Public image reads are anonymous, including cross-origin consumers.
              'Access-Control-Allow-Origin': '*',
              'Cache-Control':
                icon.kind === 'unavailable'
                  ? 'no-store'
                  : 'public, max-age=0, s-maxage=300, stale-while-revalidate=60',
              'X-Content-Type-Options': 'nosniff',
            },
          });
        } catch (error) {
          if (error instanceof Error) return unavailable();
          throw error;
        }
      }
      default: {
        const exhaustive: never = result;
        throw new TypeError(`Unexpected site lookup result: ${JSON.stringify(exhaustive)}`);
      }
    }
  };
}
