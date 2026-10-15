const messages: Readonly<Record<string, string>> = {
  username_taken: '该用户名已被使用，请换一个。',
  invalid_username: '用户名需为 3 至 32 位小写字母、数字或下划线。',
  invalid_display_name: '显示名称需为 1 至 128 个字符。',
  username_changes_exhausted: '用户名修改次数已用完。',
  profile_already_confirmed: '资料已确认，请刷新后查看。',
  profile_confirmation_required: '请先完成首次资料确认。',
  forbidden: '暂时无法保存，请刷新页面后重试。',
  unauthenticated: '登录已过期，请重新登录后查看资料。',
  session_expired: '登录已过期，请重新登录后查看资料。',
  invalid_token: '登录已过期，请重新登录后查看资料。',
  rate_limited: '操作较频繁，请稍后再试。',
};

export function profileError(code: string, uncertain: boolean): string {
  return uncertain
    ? '暂时无法确认保存结果，请刷新查看最新资料。'
    : (messages[code] ?? '保存失败，请检查输入后重试。');
}
