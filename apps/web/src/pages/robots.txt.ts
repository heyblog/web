import type { APIRoute } from 'astro';

import { robotsText } from '@/shared/crawlers';

export const prerender = true;
export const GET: APIRoute = () =>
  new Response(robotsText(), {
    headers: {
      'Content-Type': 'text/plain; charset=utf-8',
      'Cache-Control': 'public, max-age=3600',
    },
  });
