export const memberStatusKeys = ['ACTIVE', 'INACTIVE', 'ALUMNI'] as const;

export type MemberStatusKey = (typeof memberStatusKeys)[number];

export const memberStatusLabels = {
  ACTIVE: '当前成员',
  INACTIVE: '暂离',
  ALUMNI: '历史成员',
} satisfies Record<MemberStatusKey, string>;

export const memberStatusOrder = {
  ACTIVE: 0,
  INACTIVE: 1,
  ALUMNI: 2,
} satisfies Record<MemberStatusKey, number>;
