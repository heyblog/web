import {
  type ApiClientSummary,
  type ApiCredential,
  type ApiKeySummary,
  isApiClientAudience,
  isApiClientScope,
} from './api-keys.types.ts';

export function isRecord(value: unknown): value is Readonly<Record<string, unknown>> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isDate(value: unknown): value is string {
  return typeof value === 'string' && Number.isFinite(Date.parse(value));
}

function nullableDate(value: unknown): value is string | null {
  return value === null || isDate(value);
}

function isKey(value: unknown): value is ApiKeySummary {
  return (
    isRecord(value) &&
    typeof value.id === 'string' &&
    typeof value.prefix === 'string' &&
    isDate(value.created_at) &&
    nullableDate(value.expires_at) &&
    nullableDate(value.last_used_at) &&
    nullableDate(value.revoked_at) &&
    (value.rotated_from_id === null || typeof value.rotated_from_id === 'string')
  );
}

function isClient(value: unknown): value is ApiClientSummary {
  return (
    isRecord(value) &&
    typeof value.id === 'string' &&
    typeof value.name === 'string' &&
    typeof value.description === 'string' &&
    isApiClientAudience(value.audience) &&
    Array.isArray(value.scopes) &&
    value.scopes.every(isApiClientScope) &&
    nullableDate(value.disabled_at) &&
    isDate(value.created_at) &&
    isDate(value.updated_at) &&
    Array.isArray(value.keys) &&
    value.keys.every(isKey)
  );
}

export function parseClients(value: unknown): readonly ApiClientSummary[] | null {
  return isRecord(value) && Array.isArray(value.clients) && value.clients.every(isClient)
    ? value.clients
    : null;
}

export function parseClient(value: unknown): ApiClientSummary | null {
  return isRecord(value) && isClient(value.client) ? value.client : null;
}

export function parseCredential(value: unknown): ApiCredential | null {
  if (!isRecord(value) || !isRecord(value.credential)) return null;
  const credential = value.credential;
  return isClient(credential.client) &&
    isKey(credential.key) &&
    typeof credential.token === 'string' &&
    credential.token.startsWith('hbk_')
    ? { client: credential.client, key: credential.key, token: credential.token }
    : null;
}
