const trimTrailingSlash = (value: string): string => value.replace(/\/+$/, '');

const normalizeHostname = (value: string): string => value.replace(/^\[|\]$/g, '').toLowerCase();

export const isLoopbackPublicWebUrl = (baseUrl: string): boolean => {
  const hostname = normalizeHostname(new URL(baseUrl).hostname);
  return hostname === 'localhost' || hostname === '::1' || hostname.startsWith('127.');
};

export const buildPublicWebUrl = (
  baseUrl: string,
  pathname: string,
  params: Record<string, string | null | undefined> = {},
): string => {
  const target = new URL(pathname, `${trimTrailingSlash(baseUrl)}/`);

  for (const [key, value] of Object.entries(params)) {
    if (value) {
      target.searchParams.set(key, value);
    }
  }

  return target.toString();
};
