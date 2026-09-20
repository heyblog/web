import { isRecord, parseClient, parseClients, parseCredential } from './api-keys.responses.ts';
import type {
  ApiClientCreatePayload,
  ApiClientUpdatePayload,
  ApiKeyIssuePayload,
} from './api-keys.types.ts';

export class ApiKeyRequestError extends Error {
  readonly status: number;
  readonly code: string;
  constructor(status: number, code: string) {
    super(messageForFailure(status, code));
    this.status = status;
    this.code = code;
  }
}

function messageForFailure(status: number, code: string): string {
  if (code === 'api_client_name_conflict') return '此名称已被使用，请更换名称。';
  if (code === 'api_client_has_active_key') return '已有有效密钥，请刷新后使用轮换操作。';
  if (code === 'api_client_disabled') return '调用方已停用，请先启用。';
  if (status === 401) return '登录已失效，请重新登录。';
  if (status === 403) return '当前账号没有管理调用凭证的权限。';
  if (status === 404) return '调用方或密钥已不存在，请刷新列表。';
  if (status === 409) return '状态已发生变化，请刷新后重试。';
  if (status === 422) return '请检查名称、调用权限和到期时间。';
  if (code === 'invalid_response') return '返回结果无法识别，请刷新列表确认操作结果。';
  return '请求未完成，请刷新列表确认结果后再操作。';
}

export function createApiKeyClient(fetcher: typeof fetch = fetch) {
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
      if (error instanceof Error) throw new ApiKeyRequestError(0, 'network_error');
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
      throw new ApiKeyRequestError(response.status, code);
    }
    return response;
  }

  async function decode<T>(response: Response, parse: (value: unknown) => T | null): Promise<T> {
    let value: unknown;
    try {
      value = await response.json();
    } catch (error) {
      if (error instanceof Error) throw new ApiKeyRequestError(0, 'invalid_response');
      throw error;
    }
    const result = parse(value);
    if (result === null) throw new ApiKeyRequestError(0, 'invalid_response');
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
