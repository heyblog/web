import { updateManagedUser } from '../../api/managed-users/managed-users.server.ts';
import type { AuthorizationOperation } from '../../api/managed-users/managed-users.types.ts';

import { usersReturnPath } from './users.model.ts';

export async function submitAuthorization(
  request: Request,
  target: { readonly id: string | undefined; readonly operation: AuthorizationOperation },
): Promise<Response> {
  let form: FormData;
  try {
    form = await request.formData();
  } catch (error) {
    if (!(error instanceof Error)) throw error;
    return Response.json(
      { code: 'validation_failed', status: 422 },
      {
        status: 422,
        headers: { 'Cache-Control': 'private, no-store' },
      },
    );
  }
  const result = await updateManagedUser(request, target, form);
  const headers = new Headers({ 'Cache-Control': 'private, no-store', Vary: 'Accept' });
  for (const cookie of result.headers.getSetCookie()) headers.append('Set-Cookie', cookie);
  if (request.headers.get('accept')?.includes('application/json')) {
    return Response.json(
      result.ok ? { user: result.value } : { code: result.code, status: result.status },
      {
        status: result.ok ? 200 : result.status,
        headers,
      },
    );
  }
  const returnPath = usersReturnPath(form.get('returnTo'));
  const destination =
    form.get('editor') === '1' && target.id
      ? `/management/users/${encodeURIComponent(target.id)}?returnTo=${encodeURIComponent(returnPath)}`
      : '/management/users';
  const location = new URL(destination, request.url);
  location.searchParams.set(result.ok ? 'status' : 'error', result.ok ? 'updated' : result.code);
  headers.set('Location', location.toString());
  return new Response(null, { status: 303, headers });
}
