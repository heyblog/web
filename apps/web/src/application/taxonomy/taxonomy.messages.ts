const messages: Readonly<Record<string, string>> = {
  forbidden: '你没有执行此操作的权限。',
  unauthorized: '登录已失效，请重新登录。',
  taxonomy_changed: '标签数据已变化，请刷新后重新操作。',
  taxonomy_revision_changed: '标签数据已变化，请刷新后重新操作。',
  preview_changed: '迁移影响已变化，请重新预览。',
  duplicate_tag_name: '此名称已存在，请使用现有名称或合并对应标签。',
  tag_name_conflict: '已存在同名标签，请重命名或合并。',
  tag_in_use: '标签仍有引用，请先迁移或改为停用。',
  protected_taxonomy: '默认“其他”分类不能执行此结构操作。',
  validation_failed: '请检查填写内容。',
  invalid_tag: '标签已不可用，请刷新后重新选择。',
  invalid_change: '请检查迁移目标和分类归属。',
  rate_limited: '操作过于频繁，请稍后重试。',
  network_error: '请求未完成，请检查连接后重试。',
  taxonomy_preview_changed: '迁移影响已变化，请重新预览。',
  taxonomy_change_blocked: '迁移条件尚未满足，请重新预览并检查目标。',
  taxonomy_conflict: '存在同名标签或分类冲突，请检查后重试。',
  tag_has_paths: '标签仍有分类路径，请先迁移分类路径。',
  tag_has_children: '标签仍有下级分类，请先迁移分类路径。',
  tag_in_audit_history: '审核历史仍引用此标签，请使用停用或合并。',
  tag_unavailable: '标签已不可用，请刷新后重新选择。',
  fallback_protected: '默认“其他”分类必须保留。',
  primary_parent_required: '请选择有效的一级分类。',
  invalid_tag_name: '请填写标签名称。',
  bad_gateway: '服务暂时不可用，请稍后重试。',
};

export function taxonomyMessage(code: string): string {
  return messages[code] ?? '操作未完成，请刷新后重试。';
}

export function taxonomyBlocker(code: string): string {
  const kind = code.split(':')[0];
  const blockers: Readonly<Record<string, string>> = {
    duplicate_sibling_name: '目标分类下存在同名标签，请先重命名或合并。',
    replacement_required: '原分类路径需要选择迁移目标。',
    assignment_conflict: '迁移目标与现有标签引用冲突，请调整目标。',
    fallback_protected: '默认“其他”分类必须保留。',
    same_level_target_required: '合并目标必须与来源标签处于同一层级。',
    primary_parent_required: '请选择有效的一级分类。',
    source_unavailable: '来源标签已不可用，请刷新后重新选择。',
    secondary_required: '只有二级分类可以调整所属一级分类。',
    target_path_missing: '目标标签缺少对应分类路径，请先完善分类。',
    different_valid_level_required: '请选择与原层级不同的目标层级。',
    duplicate_replacement: '同一分类路径只能选择一个迁移目标。',
  };
  return blockers[kind ?? ''] ?? '迁移条件尚未满足，请检查目标及分类路径。';
}
