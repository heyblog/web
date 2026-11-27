import { authErrorMessage } from './auth-page.ts';

const messages: Readonly<Record<string, string>> = {
  email_change_active: '已有进行中的邮箱更换申请。',
  email_change_expired: '这次申请已过期，请重新发起。',
  email_change_missing: '这次申请已结束，请查看最新状态。',
  email_change_password_unverified:
    '当前密码无法用于更换邮箱。请选择原邮箱验证码或已绑定的第三方账号。',
  email_unchanged: '新邮箱与当前邮箱相同。',
  email_not_verified: '请先验证当前邮箱，再申请更换。',
  invalid_current_password: '当前密码不正确，请重新输入。',
  invalid_verification_code: '验证码无效，请检查输入或重新发送。',
  mail_unavailable: '邮件服务暂时不可用，请稍后重新发送验证码。',
  email_change_verification_required: '请先完成账号身份和新邮箱验证。',
  email_change_sites_required: '站点尚未全部复验，请查看最新列表。',
  unavailable: '暂时无法读取换绑进度，请刷新状态。',
};

export function emailChangeMessage(code: string): string {
  return messages[code] ?? authErrorMessage(code) ?? '操作未完成，请稍后重试。';
}
