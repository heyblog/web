const messages: Readonly<Record<string, string>> = {
  oauth_popup_blocked: '浏览器拦截了授权窗口，请允许弹窗后重试，或在当前页面继续。',
  oauth_popup_closed: '尚未收到授权结果，可以继续等待或重新授权。',
  oauth_timeout: '授权已超时，请重新发起。',
  oauth_cancelled: '授权已取消，可以重新发起。',
  oauth_code_required: '未收到授权码，请重新授权。',
  oauth_failed: '第三方授权未完成，请重新授权。',
  blogsclub_account_conflict: '该 BlogsClub 账号已绑定其他用户。',
  blogsclub_already_bound: '当前账号已绑定其他 BlogsClub 账号。',
  blogsclub_not_bound: '请先绑定 BlogsClub 账号。',
  blogsclub_site_unverified: '请先在 BlogsClub 完成博客认证，再重新授权。',
  blogsclub_site_mismatch: 'BlogsClub 已认证的博客地址与本次申请地址不一致。',
  oauth_authorization_required: '请通过 BlogsClub 重新授权，完成这次认证。',
  claim_unavailable: '申请已失效或不可用，请返回申请列表确认。',
  oauth_denied: '已取消授权，可以重新发起。',
  access_denied: '已取消授权，可以重新发起。',
  invalid_oauth_state: '授权已失效，请重新发起。',
  oauth_actor_changed: '登录状态已变化，请刷新页面后重新授权。',
  oauth_account_conflict: '该第三方账号已绑定其他用户。',
  oauth_already_bound: '当前账号已绑定该平台的其他账号。',
  oauth_bind_email_mismatch: '第三方账号邮箱与当前账号邮箱不一致，请使用相同邮箱的账号授权。',
  oauth_unavailable: '第三方登录暂时不可用，请稍后重试。',
  oauth_email_required: '未能取得第三方账号邮箱，请允许邮箱授权后重试。',
  blogsclub_unavailable: 'BlogsClub 登录暂时不可用，请稍后重试。',
  blogsclub_exchange_failed: 'BlogsClub 授权未完成，请重新授权。',
  blogsclub_identity_failed: '暂时无法读取 BlogsClub 账号，请稍后重试。',
  blogsclub_email_required: '未能取得 BlogsClub 邮箱，请允许邮箱授权后重试。',
  blogsclub_bind_email_mismatch: 'BlogsClub 邮箱与当前账号邮箱不一致，请使用相同邮箱的账号授权。',
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
  password_required: '解绑第三方账号前请先设置本地密码。',
  forbidden: '当前账号没有执行此操作的权限。',
  mail_unavailable: '邮件服务暂时不可用，请稍后在验证页面重新发送。',
  rate_limited: '请求过于频繁，请稍后再试。',
  bad_gateway: '登录服务暂时不可用，请稍后重试。',
  request_failed: '操作未完成，请检查输入后重试。',
};

export function authErrorMessage(code: string | null): string | null {
  return code ? (messages[code] ?? messages.request_failed ?? null) : null;
}

export function authStatusMessage(status: string | null): string | null {
  if (status === 'verified') return '邮箱验证成功，现在可以登录。';
  if (status === 'reset-sent') return '如果该邮箱对应可用账号，重置链接已发送，有效期为 30 分钟。';
  if (status === 'password-reset') return '密码已更新，请使用新密码登录。';
  if (status === 'verification-sent') return '验证码已发送，有效期为 10 分钟，请检查邮箱。';
  if (status === 'password-updated') return '密码已更新。';
  if (status === 'oauth-updated') return '第三方账号绑定状态已更新。';
  return null;
}
