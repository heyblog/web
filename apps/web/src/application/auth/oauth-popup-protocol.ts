export const oauthMessageType = 'heyblog-oauth';
export const oauthReadyMessageType = 'heyblog-oauth-ready';
export const oauthChannelName = (state: string) => `heyblog-oauth:${state}`;

export function isOAuthPopupReady(data: unknown, attempt: string): boolean {
  return (
    typeof data === 'object' &&
    data !== null &&
    'type' in data &&
    data.type === oauthReadyMessageType &&
    'attempt' in data &&
    data.attempt === attempt
  );
}
