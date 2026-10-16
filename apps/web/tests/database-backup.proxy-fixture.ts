import type { requestAuthAPI } from '../src/api/auth/auth.server.ts';

export const configuration = {
  apiBaseUrl: 'https://internal.example.test',
  apiWebToken: 't'.repeat(32),
};
export const headers = {
  Cookie: 'heyblog_access_token=synthetic',
  Origin: 'https://web.example.test',
  'Sec-Fetch-Site': 'same-origin',
  'Content-Type': 'application/json',
};
export function request(overrides: Readonly<Record<string, string>> = {}) {
  return new Request('https://web.example.test/api/web/management/database-backup/inspect', {
    method: 'POST',
    headers: { ...headers, ...overrides },
    body: JSON.stringify({ upload_id: '019f033c-2111-7000-9000-000000000004' }),
  });
}
export const authenticate: typeof requestAuthAPI = async () =>
  Response.json(
    { user: { role: 'SYS_ADMIN' } },
    { headers: { 'Set-Cookie': 'heyblog_access_token=renewed; HttpOnly; Path=/' } },
  );
