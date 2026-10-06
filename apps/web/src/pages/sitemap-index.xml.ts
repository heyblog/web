import type { APIRoute } from 'astro';

export const prerender = false;
export const GET: APIRoute = () =>
  new Response(null, { status: 308, headers: { Location: '/sitemap.xml' } });
export const HEAD = GET;
