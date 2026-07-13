const DEFAULT_API_BASE_URL = 'http://127.0.0.1:9201';

const trimTrailingSlash = (value: string): string => value.replace(/\/+$/, '');

export const getApiBaseUrl = (): string =>
  trimTrailingSlash(process.env.WEB_API_BASE_URL || DEFAULT_API_BASE_URL);

export const getWebPublicBaseUrl = (): string | null => {
  const value = process.env.WEB_PUBLIC_BASE_URL?.trim();
  return value ? trimTrailingSlash(value) : null;
};

export const getConfiguredApiBaseUrl = (): string => '';
