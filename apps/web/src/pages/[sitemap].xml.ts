import type { APIRoute } from 'astro';

import { serveSitemap } from '@/application/seo/sitemap.server';

export const prerender = false;
export const GET: APIRoute = ({ request, url }) => serveSitemap(url.pathname, request);
export const HEAD = GET;
