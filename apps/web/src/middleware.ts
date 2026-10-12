import { defineMiddleware } from 'astro:middleware';

import { readSessionUser, SessionUnavailableError } from './application/auth/auth.server';
import { isAccountPage, profileRedirect } from './application/auth/profile-routing';
import { isIndexablePath, listingSeo } from './shared/indexing';

export const onRequest = defineMiddleware(async (context, next) => {
  if (
    isAccountPage(context.url.pathname) ||
    context.url.pathname === '/complete-profile' ||
    context.url.pathname === '/login'
  ) {
    try {
      context.locals.sessionUser = await readSessionUser(context.request);
    } catch (error) {
      if (!(error instanceof SessionUnavailableError)) throw error;
      return new Response('暂时无法读取登录状态，请稍后刷新重试。', {
        status: 503,
        headers: {
          'Content-Type': 'text/plain; charset=utf-8',
          'Cache-Control': 'private, no-store',
          'Retry-After': '5',
          'X-Robots-Tag': 'noindex, follow',
        },
      });
    }
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
    // Native redirects and fetch responses can have immutable headers.
    const indexedResponse = new Response(response.body, response);
    indexedResponse.headers.set('X-Robots-Tag', 'noindex, follow');
    return indexedResponse;
  }
  return response;
});
