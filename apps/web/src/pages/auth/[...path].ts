import type { APIRoute } from 'astro';

import {
  copySetCookie,
  pageLocation,
  readProblemCode,
  requestAuthAPI,
  resolveOAuthLocation,
  safeNext,
} from '@/application/auth/auth.server';

export const prerender = false;

type FormAction = Readonly<{
  upstream: `/${string}`;
  body: (form: FormData) => Readonly<Record<string, unknown>>;
  success: (request: Request, form: FormData) => string;
}>;

const actions: Readonly<Record<string, FormAction>> = {
  login: {
    upstream: '/auth/login',
    body: (form) => ({ identifier: field(form, 'identifier'), password: field(form, 'password') }),
    success: (_request, form) => safeNext(form.get('next')),
  },
  register: {
    upstream: '/auth/register',
    body: (form) => {
      const password = field(form, 'password');
      return { username: field(form, 'username'), email: field(form, 'email'), password };
    },
    success: (_request, form) =>
      `/verify-email?status=verification-sent&email=${encodeURIComponent(field(form, 'email'))}`,
  },
  'verify-email': {
    upstream: '/auth/verify-email',
    body: (form) => ({ email: field(form, 'email'), code: field(form, 'code') }),
    success: () => '/login?status=verified',
  },
  'resend-verification': {
    upstream: '/auth/verify-email/resend',
    body: (form) => ({ email: field(form, 'email') }),
    success: (_request, form) =>
      `/verify-email?status=verification-sent&email=${encodeURIComponent(field(form, 'email'))}`,
  },
  'password/forgot': {
    upstream: '/auth/password/forgot',
    body: (form) => ({ email: field(form, 'email') }),
    success: () => '/forgot-password?status=reset-sent',
  },
  'password/reset': {
    upstream: '/auth/password/reset',
    body: (form) => {
      const password = field(form, 'password');
      return { token: field(form, 'token'), password };
    },
    success: () => '/login?status=password-reset',
  },
  password: {
    upstream: '/auth/password',
    body: (form) => {
      const nextPassword = field(form, 'next_password');
      return {
        current_password: optionalField(form, 'current_password'),
        next_password: nextPassword,
      };
    },
    success: () => '/dashboard?status=password-updated',
  },
  logout: { upstream: '/auth/logout', body: () => ({}), success: () => '/login' },
  'github/unbind': {
    upstream: '/auth/github/unbind',
    body: () => ({}),
    success: () => '/dashboard?status=github-updated',
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
      path === 'register' ? '/register' : path === 'password' ? '/dashboard' : '/reset-password';
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
              : path === 'password' || path === 'github/unbind'
                ? '/dashboard'
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
  const headers = new Headers({ Location: action.success(request, form) });
  copySetCookie(response, headers);
  return new Response(null, { status: 303, headers });
};

export const GET: APIRoute = async ({ params, request }) => {
  const path = params.path ?? '';
  if (path !== 'github/start' && path !== 'github/callback')
    return new Response('Not Found', { status: 404 });
  const incoming = new URL(request.url);
  const upstreamPath = `/auth/${path}${incoming.search}` as `/${string}`;
  const response = await requestAuthAPI(request, upstreamPath);
  if (response.status !== 302) {
    const code = await readProblemCode(response);
    const fallback =
      path === 'github/callback' && incoming.searchParams.get('intent') === 'bind'
        ? '/dashboard'
        : '/login';
    return Response.redirect(
      pageLocation(request, `${fallback}?error=${encodeURIComponent(code)}`),
      302,
    );
  }
  const location = response.headers.get('location');
  if (!location)
    return Response.redirect(pageLocation(request, '/login?error=request_failed'), 302);
  const target = resolveOAuthLocation(request, path, location);
  if (!target) {
    return Response.redirect(pageLocation(request, '/login?error=request_failed'), 302);
  }
  const headers = new Headers({ Location: target });
  copySetCookie(response, headers);
  return new Response(null, { status: 302, headers });
};

function field(form: FormData, name: string): string {
  const value = form.get(name);
  return typeof value === 'string' ? value : '';
}

function optionalField(form: FormData, name: string): string | null {
  const value = field(form, name).trim();
  return value || null;
}
