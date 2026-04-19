import type { APIRoute } from 'astro';

import { getApiBaseUrl } from '@/application/auth/auth.server';

export const prerender = false;

export const POST: APIRoute = async ({ request, redirect }) => {
  const formData = await request.formData();
  const kind = formData.get('kind');
  const id = formData.get('id');
  const decision = formData.get('decision');
  const reviewerComment = formData.get('reviewer_comment');

  if (
    (kind !== 'site' && kind !== 'article') ||
    typeof id !== 'string' ||
    (decision !== 'APPROVED' && decision !== 'REJECTED') ||
    (reviewerComment !== null && typeof reviewerComment !== 'string')
  ) {
    return new Response('Invalid feedback review request', { status: 400 });
  }

  const response = await fetch(`${getApiBaseUrl()}/api/management/feedback/${kind}/${id}/review`, {
    method: 'POST',
    headers: {
      accept: 'application/json',
      'content-type': 'application/json',
      cookie: request.headers.get('cookie') ?? '',
    },
    body: JSON.stringify({
      decision,
      reviewer_comment: reviewerComment || null,
    }),
  });

  if (response.status === 401) {
    return redirect('/login?next=%2Fmanagement%2Ffeedback', 302);
  }

  if (response.status === 403) {
    return redirect('/forbidden', 302);
  }

  if (!response.ok) {
    return new Response('Failed to review feedback', { status: response.status });
  }

  return redirect('/management/feedback', 302);
};
