const DEFAULT_API_BASE_URL = 'http://127.0.0.1:9201';

const trimTrailingSlash = (value: string): string => value.replace(/\/+$/, '');

export const getApiBaseUrl = (): string =>
  trimTrailingSlash(import.meta.env.PUBLIC_API_BASE_URL || DEFAULT_API_BASE_URL);

export const getConfiguredApiBaseUrl = (): string => {
  const raw = import.meta.env.PUBLIC_API_BASE_URL?.trim() ?? '';
  return raw ? trimTrailingSlash(raw) : '';
};
