import { isRecord, parseClient, parseClients, parseCredential } from './api-keys.responses.ts';
import type {
  ApiClientCreatePayload,
  ApiClientUpdatePayload,
  ApiKeyIssuePayload,
} from './api-keys.types.ts';

export class ApiKeyTransportError extends Error {
  readonly status: number;
  readonly code: string;
  constructor(status: number, code: string) {
    super(code);
    this.status = status;
    this.code = code;
  }
}

export function createApiKeyTransport(fetcher: typeof fetch = fetch) {
  async function request(path: string, body?: object, method = 'POST'): Promise<Response> {
    let response: Response;
    try {
      response = await fetcher(`/management/api-keys${path}`, {
        method,
        credentials: 'same-origin',
        cache: 'no-store',
        headers: body
          ? { 'Content-Type': 'application/json', Accept: 'application/json' }
          : { Accept: 'application/json' },
        body: body ? JSON.stringify(body) : undefined,
        signal: AbortSignal.timeout(15_000),
      });
    } catch (error) {
      if (error instanceof Error) throw new ApiKeyTransportError(0, 'network_error');
      throw error;
    }
    if (!response.ok) {
      let code = 'request_failed';
      try {
        const problem: unknown = await response.json();
        if (isRecord(problem) && typeof problem.code === 'string') code = problem.code;
      } catch (error) {
        if (!(error instanceof Error)) throw error;
      }
      throw new ApiKeyTransportError(response.status, code);
    }
    return response;
  }

  async function decode<T>(response: Response, parse: (value: unknown) => T | null): Promise<T> {
    let value: unknown;
    try {
      value = await response.json();
    } catch (error) {
      if (error instanceof Error) throw new ApiKeyTransportError(0, 'invalid_response');
      throw error;
    }
    const result = parse(value);
    if (result === null) throw new ApiKeyTransportError(0, 'invalid_response');
    return result;
  }

  return {
    list: async () => decode(await request('/data', undefined, 'GET'), parseClients),
    create: async (body: ApiClientCreatePayload) =>
      decode(await request('/create', body), parseCredential),
    update: async (id: string, body: ApiClientUpdatePayload) =>
      decode(await request(`/client/${encodeURIComponent(id)}`, body), parseClient),
    rotate: async (id: string, hours: number) =>
      decode(
        await request(`/client/${encodeURIComponent(id)}/rotate`, { overlap_hours: hours }),
        parseCredential,
      ),
    issue: async (id: string, body: ApiKeyIssuePayload) =>
      decode(await request(`/client/${encodeURIComponent(id)}/keys`, body), parseCredential),
    revoke: async (id: string) => {
      await request(`/key/${encodeURIComponent(id)}/revoke`);
    },
  };
}
