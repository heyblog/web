import {
  type ApiClientAudience,
  type ApiClientScope,
  apiClientScopesByAudience,
  type ApiClientSummary,
  type ApiKeyIssuePayload,
  type ApiKeySummary,
} from './api-keys.types.ts';

export type KeyStatus = 'revoked' | 'expired' | 'disabled' | 'active';

export function keyStatus(key: ApiKeySummary, disabled: boolean, now: number): KeyStatus {
  if (key.revoked_at !== null) return 'revoked';
  if (key.expires_at !== null && Date.parse(key.expires_at) <= now) return 'expired';
  return disabled ? 'disabled' : 'active';
}

export function activeKeyCount(client: ApiClientSummary, now: number): number {
  return client.keys.filter((key) => keyStatus(key, false, now) === 'active').length;
}

export function applicableScopes(
  scopes: readonly ApiClientScope[],
  audience: ApiClientAudience,
): ApiClientScope[] {
  return scopes.filter((scope) => apiClientScopesByAudience[audience].includes(scope));
}

export function defaultExpiry(audience: ApiClientAudience, now = Date.now()): string {
  const date = new Date(now + (audience === 'INTERNAL' ? 90 : 365) * 86_400_000);
  return new Date(date.getTime() - date.getTimezoneOffset() * 60_000).toISOString().slice(0, 16);
}

export function expirationPayload(
  audience: ApiClientAudience,
  value: ApiKeyIssuePayload,
  now: number,
): ApiKeyIssuePayload | null {
  if (value.never_expires)
    return audience === 'EXTERNAL' && value.expires_at === null ? value : null;
  const expiration = value.expires_at === null ? NaN : Date.parse(value.expires_at);
  if (!Number.isFinite(expiration) || expiration <= now) return null;
  if (audience === 'INTERNAL' && expiration > now + 90 * 86_400_000) return null;
  return value;
}

const dateFormatter = new Intl.DateTimeFormat('zh-CN', {
  dateStyle: 'medium',
  timeStyle: 'short',
  timeZone: 'Asia/Shanghai',
});

export function formatCredentialDate(value: string | null, empty = '尚未使用'): string {
  return value === null ? empty : dateFormatter.format(new Date(value));
}

export function lastClientUse(client: ApiClientSummary): string | null {
  return client.keys.reduce<string | null>(
    (latest, key) =>
      key.last_used_at !== null &&
      (latest === null || Date.parse(key.last_used_at) > Date.parse(latest))
        ? key.last_used_at
        : latest,
    null,
  );
}

export interface ClientFilters {
  readonly query: string;
  readonly audience: 'ALL' | ApiClientAudience;
  readonly status: 'ALL' | 'enabled' | 'disabled';
}

export function filterClients(
  clients: readonly ApiClientSummary[],
  filters: ClientFilters,
): readonly ApiClientSummary[] {
  const query = filters.query.trim().toLocaleLowerCase();
  return clients
    .filter(
      (client) =>
        (filters.audience === 'ALL' || client.audience === filters.audience) &&
        (filters.status === 'ALL' ||
          (filters.status === 'enabled') === (client.disabled_at === null)) &&
        `${client.name} ${client.description}`.toLocaleLowerCase().includes(query),
    )
    .toSorted(
      (a, b) => Date.parse(b.created_at) - Date.parse(a.created_at) || b.id.localeCompare(a.id),
    );
}
