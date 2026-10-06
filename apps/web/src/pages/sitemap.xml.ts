import type { APIRoute } from 'astro';

import { serveSitemap } from '@/application/seo/sitemap.server';

export const prerender = false;
export const GET: APIRoute = ({ request }) => serveSitemap('/sitemap.xml', request);
export const HEAD = GET;
