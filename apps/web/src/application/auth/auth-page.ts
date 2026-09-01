const messages: Readonly<Record<string, string>> = {
  invalid_credentials: '用户名、邮箱或密码不正确。',
  email_not_verified: '请先完成邮箱验证。',
  email_taken: '该邮箱已经注册。',
  username_taken: '该用户名已被使用。',
  invalid_username: '用户名需为 3 至 32 位小写字母、数字或下划线。',
  invalid_email: '请输入有效的邮箱地址。',
  invalid_password: '密码长度需为 8 至 128 个字符。',
  password_mismatch: '两次输入的密码不一致。',
  invalid_verification_code: '验证码不正确，请重新输入。',
  expired_verification_code: '验证码已过期，请重新发送。',
  verification_attempts_exceeded: '尝试次数过多，请重新发送验证码。',
  invalid_password_reset_token: '密码重置链接无效。',
  expired_password_reset_token: '密码重置链接已过期。',
  github_bind_email_mismatch: 'GitHub 主邮箱与当前账号邮箱不一致。',
  github_account_conflict: '该 GitHub 账号已绑定其他用户。',
  github_already_bound: '当前账号已绑定其他 GitHub 账号。',
  password_required: '解绑 GitHub 前请先设置本地密码。',
  forbidden: '当前账号没有执行此操作的权限。',
  mail_unavailable: '邮件服务暂时不可用，请稍后在验证页面重新发送。',
  bad_gateway: '登录服务暂时不可用，请稍后重试。',
  request_failed: '操作未完成，请检查输入后重试。',
};

export function authErrorMessage(code: string | null): string | null {
  return code ? (messages[code] ?? messages.request_failed ?? null) : null;
}

export function authStatusMessage(status: string | null): string | null {
  if (status === 'verified') return '邮箱验证成功，现在可以登录。';
  if (status === 'reset-sent') return '如果该邮箱对应可用账号，重置链接已发送。';
  if (status === 'password-reset') return '密码已更新，请使用新密码登录。';
  if (status === 'verification-sent') return '验证码已发送，请检查邮箱。';
  if (status === 'password-updated') return '密码已更新。';
  if (status === 'github-updated') return 'GitHub 绑定状态已更新。';
  return null;
}
