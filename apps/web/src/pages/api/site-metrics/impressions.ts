import type { APIRoute } from 'astro';

import { handleImpressions } from '@/api/site-metrics/site-metrics.server';

export const prerender = false;
export const ALL: APIRoute = ({ request }) => handleImpressions(request);
