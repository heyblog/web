import type { APIRoute } from 'astro';

import { createGithubAvatarHandler } from '../../../integrations/github-avatar/github-avatar.server.ts';

const handleGithubAvatar = createGithubAvatarHandler();

export const prerender = false;

export const GET = (async ({ params, request }) => {
  return handleGithubAvatar(params.username, request);
}) satisfies APIRoute;
