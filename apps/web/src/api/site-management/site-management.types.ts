export {
  type Claim,
  type ClaimAttachment,
  type ClaimAttachmentUpload,
  type ClaimChallenge,
  type ClaimMethod,
  claimMethods,
  type ClaimStatus,
  claimStatuses,
  parseChallenge,
  parseClaim,
  parseClaimAttachment,
  parseClaimAttachmentUpload,
  parseClaims,
} from './claims.types.ts';
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
