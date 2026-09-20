export const apiClientAudiences = ['INTERNAL', 'EXTERNAL'] as const;
export type ApiClientAudience = (typeof apiClientAudiences)[number];

export const apiClientScopes = ['example.call', 'data_import.write'] as const;
export type ApiClientScope = (typeof apiClientScopes)[number];

export const apiClientScopesByAudience: Readonly<
  Record<ApiClientAudience, readonly ApiClientScope[]>
> = {
  INTERNAL: ['example.call', 'data_import.write'],
  EXTERNAL: ['example.call'],
};

export const apiClientScopeLabels: Readonly<Record<ApiClientScope, string>> = {
  'data_import.write': '数据导入',
  'example.call': '示例接口调用',
};

export interface ApiClientCreatePayload {
  readonly name: string;
  readonly description: string;
  readonly audience: ApiClientAudience;
  readonly scopes: readonly ApiClientScope[];
  readonly expires_at: string | null;
  readonly never_expires: boolean;
}

export interface ApiClientUpdatePayload {
  readonly name: string;
  readonly description: string;
  readonly scopes: readonly ApiClientScope[];
  readonly enabled: boolean;
}

export interface ApiKeyIssuePayload {
  readonly expires_at: string | null;
  readonly never_expires: boolean;
}

export interface ApiKeySummary {
  readonly id: string;
  readonly prefix: string;
  readonly expires_at: string | null;
  readonly last_used_at: string | null;
  readonly created_at: string;
  readonly revoked_at: string | null;
  readonly rotated_from_id: string | null;
}

export interface ApiClientSummary {
  readonly id: string;
  readonly name: string;
  readonly description: string;
  readonly audience: ApiClientAudience;
  readonly scopes: readonly ApiClientScope[];
  readonly disabled_at: string | null;
  readonly created_at: string;
  readonly updated_at: string;
  readonly keys: readonly ApiKeySummary[];
}

export interface ApiCredential {
  readonly client: ApiClientSummary;
  readonly key: ApiKeySummary;
  readonly token: string;
}

export interface ApiClientsResponse {
  readonly clients: readonly ApiClientSummary[];
}

export interface ApiCredentialResponse {
  readonly credential: ApiCredential;
}

export interface ApiClientResponse {
  readonly client: ApiClientSummary;
}

export function parseApiClientCreatePayload(value: unknown): ApiClientCreatePayload | null {
  if (!isRecord(value)) return null;
  if (
    typeof value.name !== 'string' ||
    typeof value.description !== 'string' ||
    !isApiClientAudience(value.audience) ||
    (value.expires_at !== null && typeof value.expires_at !== 'string') ||
    typeof value.never_expires !== 'boolean'
  )
    return null;
  const audience = value.audience;
  const scopes = parseScopes(value.scopes);
  if (
    scopes === null ||
    !scopes.every((scope) => apiClientScopesByAudience[audience].includes(scope))
  )
    return null;
  return {
    name: value.name,
    description: value.description,
    audience,
    scopes,
    expires_at: value.expires_at,
    never_expires: value.never_expires,
  };
}

export function parseApiClientUpdatePayload(value: unknown): ApiClientUpdatePayload | null {
  if (!isRecord(value)) return null;
  const scopes = parseScopes(value.scopes);
  if (
    typeof value.name !== 'string' ||
    typeof value.description !== 'string' ||
    scopes === null ||
    typeof value.enabled !== 'boolean'
  )
    return null;
  return {
    name: value.name,
    description: value.description,
    scopes,
    enabled: value.enabled,
  };
}

export function isApiClientScope(value: unknown): value is ApiClientScope {
  return value === 'data_import.write' || value === 'example.call';
}

export function defaultApiClientScopes(audience: ApiClientAudience): ApiClientScope[] {
  return apiClientScopesByAudience[audience].filter((scope) => scope === 'example.call');
}

export function selectedApiClientScopes(data: FormData): readonly ApiClientScope[] {
  return data.getAll('scopes').filter(isApiClientScope);
}

export function isApiClientAudience(value: unknown): value is ApiClientAudience {
  return value === 'INTERNAL' || value === 'EXTERNAL';
}

export function parseApiKeyIssuePayload(value: unknown): ApiKeyIssuePayload | null {
  if (!isRecord(value) || typeof value.never_expires !== 'boolean') return null;
  if (value.never_expires) {
    return value.expires_at === null ? { expires_at: null, never_expires: true } : null;
  }
  if (typeof value.expires_at !== 'string' || !Number.isFinite(Date.parse(value.expires_at)))
    return null;
  return { expires_at: value.expires_at, never_expires: false };
}

function parseScopes(value: unknown): readonly ApiClientScope[] | null {
  if (
    !Array.isArray(value) ||
    value.length === 0 ||
    !value.every(isApiClientScope) ||
    new Set(value).size !== value.length
  )
    return null;
  return value;
}

function isRecord(value: unknown): value is Readonly<Record<string, unknown>> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}
