import {
  ApiKeyTransportError,
  createApiKeyTransport,
} from '../../api/api-keys/api-keys.browser.ts';
import type {
  ApiClientCreatePayload,
  ApiClientUpdatePayload,
  ApiKeyIssuePayload,
} from '../../api/api-keys/api-keys.types.ts';

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
  const transport = createApiKeyTransport(fetcher);
  async function displayFailure<T>(operation: Promise<T>): Promise<T> {
    try {
      return await operation;
    } catch (error) {
      if (error instanceof ApiKeyTransportError)
        throw new ApiKeyRequestError(error.status, error.code);
      throw error;
    }
  }
  return {
    list: () => displayFailure(transport.list()),
    create: async (body: ApiClientCreatePayload) => displayFailure(transport.create(body)),
    update: async (id: string, body: ApiClientUpdatePayload) =>
      displayFailure(transport.update(id, body)),
    rotate: async (id: string, hours: number) => displayFailure(transport.rotate(id, hours)),
    issue: async (id: string, body: ApiKeyIssuePayload) =>
      displayFailure(transport.issue(id, body)),
    revoke: async (id: string) => displayFailure(transport.revoke(id)),
  };
}
