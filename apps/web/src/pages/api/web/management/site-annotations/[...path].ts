import type { APIRoute } from 'astro';

import { forwardAnnotation } from '@/api/site-annotation/annotation.server';

export const prerender = false;
export const ALL: APIRoute = ({ request, params }) => forwardAnnotation(request, params.path ?? '');
