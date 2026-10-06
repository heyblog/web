import { defineMiddleware } from 'astro:middleware';

import { isIndexablePath, listingSeo } from './shared/indexing';

export const onRequest = defineMiddleware(async (context, next) => {
  const response = await next();
  if (
    !isIndexablePath(context.url.pathname) ||
    !listingSeo(context.url.pathname, context.url.search).indexable ||
    response.status >= 400
  ) {
    response.headers.set('X-Robots-Tag', 'noindex, follow');
  }
  return response;
});
