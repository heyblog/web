import { loadWebServerConfig, type WebServerConfig } from '../../config.server.ts';

import { apiWebTokenHeader } from './endpoint.server.ts';

export type ApiJsonResult<T> =
  | { kind: 'success'; data: T }
  | { kind: 'bad-request' }
  | { kind: 'not-found' }
  | { kind: 'unavailable' };

export interface ApiJsonDependencies {
  fetch?: typeof fetch;
  loadConfig?: () => WebServerConfig;
}

interface ApiJsonOptions extends ApiJsonDependencies {
  signal?: AbortSignal;
  timeoutMs?: number;
}

export async function fetchApiJson<T>(
  path: `/${string}`,
  options: ApiJsonOptions = {},
): Promise<ApiJsonResult<T>> {
  if (!path.startsWith('/') || path.startsWith('//') || path.includes('\\') || path.includes('#')) {
    throw new Error('API path must be an absolute path without a fragment');
  }

  let configuration: WebServerConfig;
  try {
    configuration = (options.loadConfig ?? loadWebServerConfig)();
  } catch {
    return { kind: 'unavailable' };
  }

  const timeoutSignal = AbortSignal.timeout(options.timeoutMs ?? 5_000);
  const signal = options.signal ? AbortSignal.any([options.signal, timeoutSignal]) : timeoutSignal;
  let response: Response;
  try {
    response = await (options.fetch ?? fetch)(new URL(path, configuration.apiBaseUrl), {
      method: 'GET',
      headers: {
        Accept: 'application/json',
        [apiWebTokenHeader]: configuration.apiWebToken,
      },
      redirect: 'error',
      signal,
    });
  } catch {
    return { kind: 'unavailable' };
  }

  if (response.status === 400) {
    return { kind: 'bad-request' };
  }
  if (response.status === 404) {
    return { kind: 'not-found' };
  }
  if (!response.ok) {
    return { kind: 'unavailable' };
  }
  try {
    return { kind: 'success', data: (await response.json()) as T };
  } catch {
    return { kind: 'unavailable' };
  }
}
