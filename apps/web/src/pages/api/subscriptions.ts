import type { APIRoute } from 'astro';

import { handleSiteSubscriptionRequest } from '@/application/site/site-subscription.server-handler';

export const prerender = false;

export const GET: APIRoute = async ({ request }) => handleSiteSubscriptionRequest(request);
