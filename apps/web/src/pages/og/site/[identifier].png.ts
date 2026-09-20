import type { APIRoute } from 'astro';

import { createSiteOgHandler } from '@/application/site-og/site-og.endpoint.server';

export const prerender = false;

const handle = createSiteOgHandler({
  render: async (content) => {
    const { renderSiteOg } = await import('@/application/site-og/site-og.assets.server');
    return renderSiteOg(content);
  },
});

export const GET: APIRoute = ({ params, request }) => handle(params.identifier ?? '', request);
