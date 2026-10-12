const messages: Readonly<Record<string, string>> = {
  user_conflict: '资料已更新，请重新加载后继续。',
  username_taken: '该用户名已被使用，请换一个。',
  email_taken: '该邮箱已被使用，请检查后重试。',
  pending_deletion: '此账号正在注销，暂时无法修改。',
  forbidden: '当前账号没有执行此操作的权限，请重新加载页面。',
  self_management_forbidden: '请前往个人工作区管理当前账号。',
  invalid_permission_target: '此用户已不是管理员，请重新加载页面。',
  permission_scope_exceeded: '提交的权限超出你的授权范围，请重新加载页面。',
  validation_failed: '请检查填写内容后重试。',
  invalid_role: '请选择普通用户或管理员。',
  invalid_permission: '权限选项无效，请重新加载页面。',
  duplicate_permission: '权限选项重复，请重新加载页面。',
  not_found: '此用户已不存在，请返回用户列表。',
  bad_gateway: '用户服务暂时不可用，请稍后重试。',
  invalid_response: '未能确认保存结果，请重新加载页面核实。',
  network_error: '连接中断，保存结果尚未确认，请重新加载页面核实。',
};

export function authorizationError(code: string, status = 0): string {
  if (status === 401) return '登录已失效，请重新登录后再操作。';
  if (status === 404) return messages.not_found ?? '';
  return (
    messages[code] ??
    (status >= 500 ? '用户服务暂时不可用，请稍后重试。' : '保存失败，请稍后重试。')
  );
}
