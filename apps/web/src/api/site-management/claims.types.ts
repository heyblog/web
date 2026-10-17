import { type ClaimAttachment, parseClaimAttachments } from './claim-attachment.types.ts';

export type { ClaimAttachment, ClaimAttachmentUpload } from './claim-attachment.types.ts';
export {
  parseClaimAttachment,
  parseClaimAttachments,
  parseClaimAttachmentUpload,
} from './claim-attachment.types.ts';

export const claimMethods = ['BLOGSCLUB', 'DNS_TXT', 'META', 'FILE', 'MANUAL'] as const;
export type ClaimMethod = (typeof claimMethods)[number];
export const claimStatuses = ['PENDING', 'VERIFIED', 'REJECTED', 'CANCELLED'] as const;
export type ClaimStatus = (typeof claimStatuses)[number];
export type OwnershipState = 'none' | 'mine' | 'other' | 'stale';
export interface ClaimPerson {
  readonly id: string;
  readonly username: string;
  readonly display_name: string;
}
export interface ClaimSite {
  readonly short_id: string;
  readonly name: string;
  readonly address: string;
  readonly visibility: string;
}
export interface Claim {
  readonly id: string;
  readonly short_id: string;
  readonly user_id: string;
  readonly address: string;
  readonly method: ClaimMethod;
  readonly status: ClaimStatus;
  readonly created_at: string;
  readonly expires_at?: string;
  readonly verified_at?: string;
  readonly consumed_at?: string;
  readonly review_reason?: string;
  readonly evidence?: string;
  readonly evidence_url?: string;
  readonly attachments?: readonly ClaimAttachment[];
  readonly site_name: string;
  readonly current_address: string;
  readonly ownership_state: OwnershipState;
  readonly can_check: boolean;
  readonly can_regenerate: boolean;
  readonly can_cancel: boolean;
  readonly can_manage: boolean;
  readonly applicant?: ClaimPerson;
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
export interface ClaimContext {
  readonly site: ClaimSite;
  readonly ownership_state: OwnershipState;
  readonly can_claim: boolean;
  readonly can_manage: boolean;
  readonly can_verify_address: boolean;
  readonly pending_claims: readonly Claim[];
}
export interface ClaimPage {
  readonly items: readonly Claim[];
  readonly has_more: boolean;
}
export interface SiteOwnership {
  readonly id: string;
  readonly user_id: string;
  readonly address: string;
  readonly revision: number;
  readonly owner: ClaimPerson;
  readonly is_current: boolean;
}
export interface OwnershipEvent {
  readonly id: string;
  readonly user_id: string | null;
  readonly actor_id: string | null;
  readonly action: string;
  readonly reason: string;
  readonly evidence: string | null;
  readonly created_at: string;
}
export interface OwnershipDetail {
  readonly site: ClaimSite;
  readonly current: SiteOwnership | null;
  readonly history: readonly OwnershipEvent[];
  readonly can_revoke: boolean;
}

export function record(value: unknown): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value))
    throw new TypeError('Invalid claim response');
  return Object.fromEntries(Object.entries(value));
}
function text(value: unknown): string {
  if (typeof value !== 'string') throw new TypeError('Invalid claim text');
  return value;
}
function optional(value: unknown): string | undefined {
  return value == null ? undefined : text(value);
}
function boolean(value: unknown): boolean {
  if (typeof value !== 'boolean') throw new TypeError('Invalid claim flag');
  return value;
}
function timestamp(value: unknown): string {
  const result = text(value);
  if (!Number.isFinite(Date.parse(result))) throw new TypeError('Invalid claim date');
  return result;
}
function optionalTime(value: unknown): string | undefined {
  return value == null ? undefined : timestamp(value);
}
function state(value: unknown): OwnershipState {
  if (value !== 'none' && value !== 'mine' && value !== 'other' && value !== 'stale')
    throw new TypeError('Invalid ownership state');
  return value;
}
function person(value: unknown): ClaimPerson {
  const item = record(value);
  return {
    id: text(item.id),
    username: text(item.username),
    display_name: text(item.display_name),
  };
}
function site(value: unknown): ClaimSite {
  const item = record(value);
  return {
    short_id: text(item.short_id),
    name: text(item.name),
    address: text(item.address),
    visibility: text(item.visibility),
  };
}
export function parseClaim(value: unknown): Claim {
  const item = record(value);
  const method = claimMethods.find((entry) => entry === item.method);
  const status = claimStatuses.find((entry) => entry === item.status);
  if (!method || !status) throw new TypeError('Invalid claim status');
  return {
    id: text(item.id),
    short_id: text(item.short_id),
    user_id: text(item.user_id),
    address: text(item.address),
    method,
    status,
    created_at: timestamp(item.created_at),
    expires_at: optionalTime(item.expires_at),
    verified_at: optionalTime(item.verified_at),
    consumed_at: optionalTime(item.consumed_at),
    review_reason: optional(item.review_reason),
    evidence: optional(item.evidence),
    evidence_url: optional(item.evidence_url),
    ...(item.attachments == null ? {} : { attachments: parseClaimAttachments(item.attachments) }),
    site_name: text(item.site_name),
    current_address: text(item.current_address),
    ownership_state: state(item.ownership_state),
    can_check: boolean(item.can_check),
    can_regenerate: boolean(item.can_regenerate),
    can_cancel: boolean(item.can_cancel),
    can_manage: boolean(item.can_manage),
    ...(item.applicant == null ? {} : { applicant: person(item.applicant) }),
  };
}
export function parseClaimPage(value: unknown): ClaimPage {
  const item = record(value);
  if (!Array.isArray(item.items)) throw new TypeError('Invalid claim list');
  return { items: item.items.map(parseClaim), has_more: boolean(item.has_more) };
}
export function parseClaims(value: unknown): readonly Claim[] {
  return parseClaimPage(value).items;
}
export function parseChallenge(value: unknown): ClaimChallenge {
  const item = record(value);
  const instructions = item.instructions == null ? null : record(item.instructions);
  return {
    claim: parseClaim(item.claim),
    token: optional(item.token),
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
export function parseClaimContext(value: unknown): ClaimContext {
  const item = record(value);
  if (!Array.isArray(item.pending_claims)) throw new TypeError('Invalid pending claims');
  return {
    site: site(item.site),
    ownership_state: state(item.ownership_state),
    can_claim: boolean(item.can_claim),
    can_manage: boolean(item.can_manage),
    can_verify_address: boolean(item.can_verify_address),
    pending_claims: item.pending_claims.map(parseClaim),
  };
}
export function parseOwnership(value: unknown): OwnershipDetail {
  const item = record(value);
  const owner = item.current == null ? null : record(item.current);
  if (
    !Array.isArray(item.history) ||
    (owner &&
      (typeof owner.revision !== 'number' ||
        !Number.isSafeInteger(owner.revision) ||
        owner.revision < 1))
  )
    throw new TypeError('Invalid ownership response');
  const revision = owner?.revision;
  const current =
    owner && typeof revision === 'number'
      ? {
          id: text(owner.id),
          user_id: text(owner.user_id),
          address: text(owner.address),
          revision,
          owner: person(owner.owner),
          is_current: boolean(owner.is_current),
        }
      : null;
  return {
    site: site(item.site),
    can_revoke: boolean(item.can_revoke),
    current,
    history: item.history.map((value: unknown) => {
      const event = record(value);
      return {
        id: text(event.id),
        user_id: event.user_id == null ? null : text(event.user_id),
        actor_id: event.actor_id == null ? null : text(event.actor_id),
        action: text(event.action),
        reason: text(event.reason),
        evidence: event.evidence == null ? null : text(event.evidence),
        created_at: timestamp(event.created_at),
      };
    }),
  };
}
