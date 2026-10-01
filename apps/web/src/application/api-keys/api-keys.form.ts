import { isApiClientScope } from '../../api/api-keys/api-keys.types.ts';
import { apiClientScopesByAudience } from '../../api/api-keys/api-keys.types.ts';
import { type ApiClientScope } from '../../api/api-keys/api-keys.types.ts';
import { type ApiClientAudience } from '../../api/api-keys/api-keys.types.ts';

export const apiClientScopeLabels: Readonly<Record<ApiClientScope, string>> = {
  'data_import.write': '数据导入',
  'example.call': '示例接口调用',
};

export function defaultApiClientScopes(audience: ApiClientAudience): ApiClientScope[] {
  return apiClientScopesByAudience[audience].filter((scope) => scope === 'example.call');
}

export function selectedApiClientScopes(data: FormData): readonly ApiClientScope[] {
  return data.getAll('scopes').filter(isApiClientScope);
}
