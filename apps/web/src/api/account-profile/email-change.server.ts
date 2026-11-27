import { copySetCookie, requestAuthAPI } from '../auth/auth.server.ts';

import { type EmailChangeStatus, parseEmailChangeStatus } from './email-change.types.ts';

export async function loadEmailChange(
  request: Request,
  responseHeaders: Headers,
): Promise<EmailChangeStatus | null> {
  const response = await requestAuthAPI(request, '/api/web/account/email-change');
  copySetCookie(response, responseHeaders);
  if (!response.ok) return null;
  try {
    const payload: unknown = await response.json();
    return parseEmailChangeStatus(payload);
  } catch (error) {
    if (!(error instanceof Error)) throw error;
    return null;
  }
}
