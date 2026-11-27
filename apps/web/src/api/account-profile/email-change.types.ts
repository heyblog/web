const emailChangeIdPattern = /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/iu;
const siteStatuses = ['required', 'pending', 'verified', 'rejected'] as const;

export type EmailChangeSide = 'old' | 'new';
export interface EmailChangeRequest {
  readonly id: string;
  readonly new_email: string;
  readonly expires_at: string;
  readonly old_verified: boolean;
  readonly new_verified: boolean;
}

export interface EmailChangeSite {
  readonly short_id: string;
  readonly name: string;
  readonly address: string;
  readonly status: (typeof siteStatuses)[number];
  readonly claim_id: string;
}

export interface EmailChangeStatus {
  readonly request: EmailChangeRequest | null;
  readonly sites: readonly EmailChangeSite[];
}

export function isEmailChangeId(value: unknown): value is string {
  return typeof value === 'string' && emailChangeIdPattern.test(value);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function parseRequest(value: unknown): EmailChangeRequest | null {
  if (
    !isRecord(value) ||
    !isEmailChangeId(value.id) ||
    typeof value.new_email !== 'string' ||
    typeof value.expires_at !== 'string' ||
    !Number.isFinite(Date.parse(value.expires_at)) ||
    typeof value.old_verified !== 'boolean' ||
    typeof value.new_verified !== 'boolean'
  )
    return null;
  return {
    id: value.id,
    new_email: value.new_email,
    expires_at: value.expires_at,
    old_verified: value.old_verified,
    new_verified: value.new_verified,
  };
}

function parseSite(value: unknown): EmailChangeSite | null {
  if (
    !isRecord(value) ||
    typeof value.short_id !== 'string' ||
    !/^[a-z0-9]{9}$/iu.test(value.short_id) ||
    typeof value.name !== 'string' ||
    typeof value.address !== 'string' ||
    (value.claim_id !== '' && !isEmailChangeId(value.claim_id))
  )
    return null;
  const status = siteStatuses.find((entry) => entry === value.status);
  if (!status || (status !== 'required' && !value.claim_id)) return null;
  return {
    short_id: value.short_id,
    name: value.name,
    address: value.address,
    status,
    claim_id: value.claim_id,
  };
}

export function parseEmailChangeStatus(value: unknown): EmailChangeStatus | null {
  if (!isRecord(value) || !Array.isArray(value.sites)) return null;
  const request = value.request === null ? null : parseRequest(value.request);
  if (value.request !== null && !request) return null;
  const sites: EmailChangeSite[] = [];
  for (const rawSite of value.sites) {
    const site = parseSite(rawSite);
    if (!site) return null;
    sites.push(site);
  }
  return { request, sites };
}
