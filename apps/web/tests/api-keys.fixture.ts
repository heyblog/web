import type {
  ApiClientSummary,
  ApiKeySummary,
} from '../src/application/api-keys/api-keys.types.ts';

export const credentialNow = Date.parse('2026-09-20T10:00:00Z');
export const credentialKey: ApiKeySummary = {
  id: '018f0d5e-7b16-7cc1-9d8e-a5f4bf44e201',
  prefix: 'hbk_fixture',
  created_at: '2026-09-01T10:00:00Z',
  expires_at: '2026-10-01T10:00:00Z',
  last_used_at: null,
  revoked_at: null,
  rotated_from_id: null,
};
export const credentialClient: ApiClientSummary = {
  id: '018f0d5e-7b16-7cc1-9d8e-a5f4bf44e101',
  name: '内容同步',
  description: '内部导入服务',
  audience: 'INTERNAL',
  scopes: ['example.call'],
  disabled_at: null,
  created_at: '2026-09-01T10:00:00Z',
  updated_at: '2026-09-01T10:00:00Z',
  keys: [credentialKey],
};
