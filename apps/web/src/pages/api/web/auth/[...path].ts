import type { APIRoute } from 'astro';

import { parseSessionResponse } from '@/api/auth/auth.responses.ts';
import { copySetCookie, readProblemCode, requestAuthAPI } from '@/api/auth/auth.server.ts';
import { isOAuthProvider } from '@/api/auth/oauth.types.ts';
import { pageLocation, safeNext } from '@/application/auth/auth.server.ts';
import { startOAuth } from '@/application/auth/oauth.server.ts';
import { confirmationLocation } from '@/application/auth/profile-routing.ts';

export const prerender = false;

type FormAction = Readonly<{
  upstream: `/${string}`;
  body: (form: FormData) => Readonly<Record<string, unknown>>;
  success: (request: Request, form: FormData) => string;
}>;

const actions: Readonly<Record<string, FormAction>> = {
  login: {
    upstream: '/api/web/auth/login',
    body: (form) => ({ identifier: field(form, 'identifier'), password: field(form, 'password') }),
    success: (_request, form) => safeNext(form.get('next')),
  },
  register: {
    upstream: '/api/web/auth/register',
    body: (form) => {
      const password = field(form, 'password');
      return { username: field(form, 'username'), email: field(form, 'email'), password };
    },
    success: (_request, form) =>
      `/verify-email?status=verification-sent&email=${encodeURIComponent(field(form, 'email'))}`,
  },
  'verify-email': {
    upstream: '/api/web/auth/verify-email',
    body: (form) => ({ email: field(form, 'email'), code: field(form, 'code') }),
    success: () => '/login?status=verified',
  },
  'resend-verification': {
    upstream: '/api/web/auth/verify-email/resend',
    body: (form) => ({ email: field(form, 'email') }),
    success: (_request, form) =>
      `/verify-email?status=verification-sent&email=${encodeURIComponent(field(form, 'email'))}`,
  },
  'password/forgot': {
    upstream: '/api/web/auth/password/forgot',
    body: (form) => ({ email: field(form, 'email') }),
    success: () => '/forgot-password?status=reset-sent',
  },
  'password/reset': {
    upstream: '/api/web/auth/password/reset',
    body: (form) => {
      const password = field(form, 'password');
      return { token: field(form, 'token'), password };
    },
    success: () => '/login?status=password-reset',
  },
  password: {
    upstream: '/api/web/auth/password',
    body: (form) => {
      const nextPassword = field(form, 'next_password');
      return {
        current_password: optionalField(form, 'current_password'),
        next_password: nextPassword,
      };
    },
    success: () => '/dashboard/security?status=password-updated',
  },
  logout: { upstream: '/api/web/auth/logout', body: () => ({}), success: () => '/login' },
  'blogsclub/unbind': {
    upstream: '/api/web/auth/blogsclub/unbind',
    body: () => ({}),
    success: () => '/dashboard/security?status=oauth-updated',
  },
  'github/unbind': {
    upstream: '/api/web/auth/github/unbind',
    body: () => ({}),
    success: () => '/dashboard/security?status=oauth-updated',
  },
};

export const POST: APIRoute = async ({ params, request }) => {
  const path = params.path ?? '';
  const action = actions[path];
  if (!action) return new Response('Not Found', { status: 404 });
  const form = await request.formData();
  const confirmation = form.get('confirm_password');
  const proposedPassword = form.get('password') ?? form.get('next_password');
  if (confirmation !== null && confirmation !== proposedPassword) {
    const fallback =
      path === 'register'
        ? '/register'
        : path === 'password'
          ? '/dashboard/security'
          : '/reset-password';
    return Response.redirect(pageLocation(request, `${fallback}?error=password_mismatch`), 303);
  }
  const body = action.body(form);
  const response = await requestAuthAPI(request, action.upstream, { method: 'POST', body });
  if (!response.ok) {
    const code = await readProblemCode(response);
    const fallback =
      path === 'register' && code === 'mail_unavailable'
        ? '/verify-email'
        : path === 'register'
          ? '/register'
          : path === 'password/forgot'
            ? '/forgot-password'
            : path.startsWith('password/reset')
              ? '/reset-password'
              : path === 'password' || path.endsWith('/unbind')
                ? '/dashboard/security'
                : path.includes('verify')
                  ? '/verify-email'
                  : '/login';
    const parameters = new URLSearchParams({ error: code });
    const email = optionalField(form, 'email');
    if (email) parameters.set('email', email);
    const token = optionalField(form, 'token');
    if (token) parameters.set('token', token);
    return Response.redirect(pageLocation(request, `${fallback}?${parameters}`), 303);
  }
  let location = action.success(request, form);
  if (path === 'login') {
    const user = parseSessionResponse(await response.json());
    if (!user) return Response.redirect(pageLocation(request, '/login?error=request_failed'), 303);
    if (user.profile_confirmation_required) location = confirmationLocation(location);
  }
  const headers = new Headers({ Location: location, 'Cache-Control': 'private, no-store' });
  copySetCookie(response, headers);
  return new Response(null, { status: 303, headers });
};

export const GET: APIRoute = async ({ params, request }) => {
  const [provider, action, extra] = (params.path ?? '').split('/');
  if (!isOAuthProvider(provider) || action !== 'start' || extra !== undefined)
    return new Response('Not Found', { status: 404 });
  return startOAuth(request, provider);
};

function field(form: FormData, name: string): string {
  const value = form.get(name);
  return typeof value === 'string' ? value : '';
}

function optionalField(form: FormData, name: string): string | null {
  const value = field(form, name).trim();
  return value || null;
}
