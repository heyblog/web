import { siteConfig } from '../../site.config.ts';

const source = new URL(siteConfig.url).hostname;

export function attributedSiteUrl(value: string): string | undefined {
  let url: URL;
  try {
    url = new URL(value);
  } catch {
    return undefined;
  }
  if ((url.protocol !== 'https:' && url.protocol !== 'http:') || url.username || url.password)
    return undefined;
  url.searchParams.set('utm_source', source);
  url.searchParams.set('utm_medium', 'referral');
  return url.href;
}

export function siteOutboundPath(shortId: string): string {
  return `/site/out/${encodeURIComponent(shortId)}`;
}

export function siteOutboundAttributes(
  target: { readonly shortId: string } | { readonly url: string },
) {
  return {
    href: 'shortId' in target ? siteOutboundPath(target.shortId) : attributedSiteUrl(target.url),
    target: '_blank',
    rel: 'noopener',
    referrerpolicy: 'origin',
    'data-astro-prefetch': 'false',
  } as const;
}
