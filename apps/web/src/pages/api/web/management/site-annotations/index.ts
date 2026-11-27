import type { APIRoute } from 'astro';

import { forwardAnnotation } from '@/api/site-annotation/annotation.server';

export const prerender = false;
export const ALL: APIRoute = ({ request }) => forwardAnnotation(request, '');
