import type {
  Claim,
  ClaimContext,
  OwnershipDetail,
} from '../src/api/site-management/claims.types.ts';
export const claimFixture = {
  id: '019f033c-2111-7000-9000-000000000001',
  short_id: 'abc123456',
  user_id: '019f033c-2111-7000-9000-000000000002',
  address: 'https://example.test/',
  method: 'DNS_TXT',
  status: 'PENDING',
  created_at: '2026-10-01T00:00:00Z',
  expires_at: '2099-10-02T00:00:00Z',
  site_name: 'Example',
  current_address: 'https://example.test/',
  ownership_state: 'none',
  can_check: true,
  can_regenerate: true,
  can_cancel: true,
  can_manage: false,
} satisfies Claim;
export const contextFixture = {
  site: {
    short_id: claimFixture.short_id,
    name: 'Example',
    address: claimFixture.address,
    visibility: 'PUBLIC',
  },
  ownership_state: 'none',
  can_claim: true,
  can_manage: false,
  can_verify_address: false,
  pending_claims: [],
} satisfies ClaimContext;
export const proofFixture = {
  claim: claimFixture,
  token: 'synthetic-token',
  instructions: {
    dns_name: '_heyblog-verification.example.test',
    dns_value: 'heyblog-site-verification=synthetic-token',
    meta: '',
    file_url: '',
    file_content: '',
  },
};
export const ownershipFixture = {
  site: contextFixture.site,
  can_revoke: true,
  current: {
    id: '019f033c-2111-7000-9000-000000000003',
    user_id: claimFixture.user_id,
    address: claimFixture.address,
    revision: 2,
    is_current: true,
    owner: { id: claimFixture.user_id, username: 'owner', display_name: 'Owner' },
  },
  history: [],
} satisfies OwnershipDetail;
