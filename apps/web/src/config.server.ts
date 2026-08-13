export interface WebServerConfig {
  apiBaseUrl: string;
  apiWebToken: string;
  githubToken?: string;
}

export type WebServerEnvironment = Readonly<Record<string, string | undefined>>;

const minimumTokenLength = 32;
const bearerToken = /^[A-Za-z0-9\-._~+/]+={0,}$/;

export function loadWebServerConfig(
  environment: WebServerEnvironment = process.env,
): WebServerConfig {
  return {
    apiBaseUrl: resolveApiBaseUrl(environment.WEB_API_BASE_URL),
    apiWebToken: resolveApiWebToken(environment.API_WEB_TOKEN),
    githubToken: resolveGithubToken(environment.GITHUB_TOKEN),
  };
}

function resolveApiBaseUrl(value: string | undefined): string {
  const rawValue = value?.trim();
  if (!rawValue) {
    throw new Error('WEB_API_BASE_URL is required');
  }

  let parsed: URL;
  try {
    parsed = new URL(rawValue);
  } catch {
    throw new Error('WEB_API_BASE_URL must be a valid HTTP service URL');
  }

  if (
    (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') ||
    !parsed.host ||
    parsed.username ||
    parsed.password ||
    parsed.pathname !== '/' ||
    parsed.search ||
    parsed.hash
  ) {
    throw new Error(
      'WEB_API_BASE_URL must be an origin without credentials, path, query, or fragment',
    );
  }

  return parsed.origin;
}

function resolveApiWebToken(value: string | undefined): string {
  if (!value || value.length < minimumTokenLength || !bearerToken.test(value)) {
    throw new Error(
      `API_WEB_TOKEN must contain at least ${minimumTokenLength} valid Bearer token characters`,
    );
  }
  return value;
}

function resolveGithubToken(value: string | undefined): string | undefined {
  const token = value?.trim();

  if (!token) {
    return undefined;
  }

  if (/\s/u.test(token)) {
    throw new Error('GITHUB_TOKEN must not contain whitespace');
  }

  return token;
}
