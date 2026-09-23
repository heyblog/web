export interface SiteGoQuery {
  level1: string;
  level2: string;
  preview: boolean;
}

export type SiteGoQueryResult =
  { kind: 'valid'; query: SiteGoQuery } | { kind: 'invalid'; code: string; preview: boolean };

export function parseSiteGoQuery(parameters: URLSearchParams): SiteGoQueryResult {
  const preview = parameters.get('preview') === 'true';
  const invalid = (code: string): SiteGoQueryResult => ({ kind: 'invalid', code, preview });
  for (const name of ['level1', 'level2', 'preview']) {
    if (parameters.getAll(name).length > 1) return invalid('random_duplicate_parameter');
  }
  for (const name of parameters.keys()) {
    if (!['level1', 'level2', 'preview'].includes(name)) return invalid('random_unknown_parameter');
  }
  if (parameters.has('preview') && !preview) return invalid('random_invalid_preview');
  for (const name of ['level1', 'level2']) {
    const value = parameters.get(name)?.trim();
    if (
      value !== undefined &&
      (!value || [...value].length > 100 || /[\p{Cc}\p{Cs}\uFFFD]/u.test(value))
    ) {
      return invalid('random_invalid_parameter');
    }
  }
  const level1 = parameters.get('level1')?.trim() ?? '';
  const level2 = parameters.get('level2')?.trim() ?? '';
  if (level2 && !level1) return invalid('random_missing_level1');
  return { kind: 'valid', query: { level1, level2, preview } };
}

export function buildSiteGoHref(level1: string, level2: string, preview = false): string {
  const parameters = new URLSearchParams();
  if (level1) parameters.set('level1', level1);
  if (level1 && level2) parameters.set('level2', level2);
  if (preview) parameters.set('preview', 'true');
  const query = parameters.toString();
  return query ? `/site/go?${query}` : '/site/go';
}

export function buildSiteGoLink(baseURL: string, level1: string, level2: string): string {
  return new URL(buildSiteGoHref(level1, level2), baseURL).href;
}

export function legacyRandomDestination(url: URL): string {
  return `/site/go${url.search}`;
}

const parameterMessages: Readonly<Record<string, string>> = {
  random_duplicate_parameter: '同一个参数只能填写一次，请删除重复的参数。',
  random_unknown_parameter: '链接包含不支持的参数。可使用 level1、level2 和 preview。',
  random_invalid_preview: '预览模式请使用 preview=true；自动跳转请移除 preview 参数。',
  random_invalid_parameter: '分类名称不能为空，且不能超过 100 个字符。不限分类时请移除对应参数。',
  random_missing_level1: '指定二级分类时，请同时填写对应的一级分类。',
  random_unknown_level1: '一级分类不存在或已停用，请从当前分类中重新选择。',
  random_unknown_level2: '二级分类不存在或已停用，请从当前分类中重新选择。',
  random_classification_mismatch: '二级分类不属于所选一级分类，请重新选择分类组合。',
};

export function siteGoParameterMessage(code?: string): string {
  return parameterMessages[code ?? ''] ?? '链接中的分类参数无效，请重新选择分类并生成链接。';
}
