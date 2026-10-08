export const claimMethods = ['DNS_TXT', 'META', 'FILE', 'MANUAL'] as const;
export type ClaimMethod = (typeof claimMethods)[number];
export const claimStatuses = ['PENDING', 'VERIFIED', 'REJECTED', 'CANCELLED'] as const;
export type ClaimStatus = (typeof claimStatuses)[number];
export interface Claim {
  readonly id: string;
  readonly short_id: string;
  readonly user_id: string;
  readonly address: string;
  readonly method: ClaimMethod;
  readonly status: ClaimStatus;
  readonly created_at: string;
  readonly expires_at?: string;
  readonly review_reason?: string;
  readonly evidence?: string;
  readonly evidence_url?: string;
}
export interface ClaimChallenge {
  readonly claim: Claim;
  readonly token?: string;
  readonly instructions?: {
    readonly dns_name: string;
    readonly dns_value: string;
    readonly meta: string;
    readonly file_url: string;
    readonly file_content: string;
  };
}
export interface FriendLink {
  readonly target_short_id: string;
  readonly target_host: string;
  readonly target_url: string;
  readonly target_name: string;
  readonly link_status: string;
  readonly is_reciprocal: boolean;
}
export interface FriendRequest {
  readonly id: string;
  readonly audit_id: string;
  readonly target_name: string;
  readonly target_url: string;
  readonly status: string;
}
export interface FriendLinks {
  readonly items: readonly FriendLink[];
  readonly pending: readonly FriendRequest[];
}

function record(value: unknown): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value))
    throw new Error('响应格式无效');
  return Object.fromEntries(Object.entries(value));
}
function text(value: unknown): string {
  if (typeof value !== 'string') throw new Error('响应格式无效');
  return value;
}
function optional(value: unknown): string | undefined {
  return value == null ? undefined : text(value);
}
export function parseClaim(value: unknown): Claim {
  const item = record(value);
  const method = claimMethods.find((entry) => entry === item.method);
  const status = claimStatuses.find((entry) => entry === item.status);
  if (!method || !status) throw new Error('响应格式无效');
  return {
    id: text(item.id),
    short_id: text(item.short_id),
    user_id: text(item.user_id),
    address: text(item.address),
    method,
    status,
    created_at: text(item.created_at),
    expires_at: optional(item.expires_at),
    review_reason: optional(item.review_reason),
    evidence: optional(item.evidence),
    evidence_url: optional(item.evidence_url),
  };
}
export function parseClaims(value: unknown): readonly Claim[] {
  const payload = record(value);
  if (!Array.isArray(payload.items)) throw new Error('响应格式无效');
  return payload.items.map(parseClaim);
}
export function parseChallenge(value: unknown): ClaimChallenge {
  const payload = record(value);
  const instructions = payload.instructions ? record(payload.instructions) : null;
  return {
    claim: parseClaim(payload.claim),
    token: optional(payload.token),
    ...(instructions
      ? {
          instructions: {
            dns_name: text(instructions.dns_name),
            dns_value: text(instructions.dns_value),
            meta: text(instructions.meta),
            file_url: text(instructions.file_url),
            file_content: text(instructions.file_content),
          },
        }
      : {}),
  };
}
export function parseFriendLinks(value: unknown): FriendLinks {
  const payload = record(value);
  if (!Array.isArray(payload.items) || !Array.isArray(payload.pending))
    throw new Error('响应格式无效');
  return {
    items: payload.items.map((value: unknown) => {
      const item = record(value);
      if (typeof item.is_reciprocal !== 'boolean') throw new Error('响应格式无效');
      return {
        target_short_id: text(item.target_short_id),
        target_host: text(item.target_host),
        target_url: text(item.target_url),
        target_name: text(item.target_name),
        link_status: text(item.link_status),
        is_reciprocal: item.is_reciprocal,
      };
    }),
    pending: payload.pending.map((value: unknown) => {
      const item = record(value);
      return {
        id: text(item.id),
        audit_id: text(item.audit_id),
        target_name: text(item.target_name),
        target_url: text(item.target_url),
        status: text(item.status),
      };
    }),
  };
}
