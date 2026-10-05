import type { APIRoute } from 'astro';

import { forwardTaxonomyManagement } from '@/api/taxonomy/taxonomy.proxy.server';
export const prerender = false;
export const ALL: APIRoute = ({ request }) => forwardTaxonomyManagement(request, 'slug-generation');
