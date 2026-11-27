import type {
  AnnotationComponent,
  AnnotationDetail,
  AnnotationItem,
  AnnotationTag,
  OutgoingLink,
} from '../../api/site-annotation/annotation.types.ts';

export const annotationListPath = '/annotation';

export function annotationDetailPath(shortId: string): string {
  return `${annotationListPath}/${encodeURIComponent(shortId)}`;
}

export const statusLabels = {
  UNREVIEWED: '待标注',
  BLOG: '博客',
  NON_BLOG: '非博客',
  ALL: '全部',
} satisfies Record<AnnotationItem['blog_status'] | 'ALL', string>;

export const scopeLabels = {
  ALL: '全球可访问',
  CN_ONLY: '仅中国大陆可访问',
  GLOBAL_ONLY: '仅中国大陆以外可访问',
} satisfies Record<AnnotationDetail['access_scope'], string>;

export const tagRoleLabels = {
  PRIMARY: '一级分类',
  SECONDARY: '二级分类',
  TERTIARY: '主题标签',
  WARNING: '提示标签',
} satisfies Record<AnnotationTag['role'], string>;

export const componentRoleLabels = {
  SITE_PROGRAM: '站点程序',
  FRAMEWORK: '框架',
  LANGUAGE: '语言',
  RUNTIME: '运行时',
  OTHER: '其他',
} satisfies Record<AnnotationComponent['role'], string>;

export const linkStatusLabels = {
  ACTIVE: '启用',
  INACTIVE: '停用',
} satisfies Record<OutgoingLink['link_status'], string>;

export const resourceLabels = { SITEMAP: '站点地图', LINK_PAGE: '友链页面' };
export const feedFormatLabels = { UNKNOWN: '未知', RSS: 'RSS', ATOM: 'Atom', JSON: 'JSON Feed' };

const dateFormat = new Intl.DateTimeFormat('zh-CN', {
  dateStyle: 'medium',
  timeStyle: 'short',
  timeZone: 'Asia/Shanghai',
});

export function annotationTime(value: string): string {
  return dateFormat.format(new Date(value));
}

export function annotationFailure(status: number, operation: 'load' | 'save'): string {
  if (status === 401) return '登录已失效，请登录后重试。';
  if (status === 403) return '你没有标注权限。';
  if (status === 404) return '未找到该站点。';
  if (operation === 'load') return '暂时无法加载，请重试。';
  if (status === 413) return '改动内容过多，请分次保存。';
  if (status === 400 || status === 422) return '请检查填写内容、重复条目和分类选项。';
  return '暂时无法确认保存结果。请查看已保存值。';
}
