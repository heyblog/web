import { defineMiddleware } from 'astro:middleware';

import { readSessionUser } from './application/auth/auth.server';
import { isAccountPage, profileRedirect } from './application/auth/profile-routing';
import { isIndexablePath, listingSeo } from './shared/indexing';

export const onRequest = defineMiddleware(async (context, next) => {
  if (
    isAccountPage(context.url.pathname) ||
    context.url.pathname === '/complete-profile' ||
    context.url.pathname === '/login'
  ) {
    context.locals.sessionUser = await readSessionUser(context.request);
    const location = profileRedirect(context.url, context.locals.sessionUser);
    if (location)
      return new Response(null, {
        status: 302,
        headers: { Location: location, 'Cache-Control': 'private, no-store' },
      });
  }
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
