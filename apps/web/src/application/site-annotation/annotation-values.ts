import {
  componentRoleLabels,
  feedFormatLabels,
  linkStatusLabels,
  resourceLabels,
  scopeLabels,
  statusLabels,
  tagRoleLabels,
} from './annotation-view.ts';

function object(value: unknown): Readonly<Record<string, unknown>> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
    ? Object.fromEntries(Object.entries(value))
    : {};
}

function string(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function label(value: unknown, labels: Readonly<Record<string, string>>): string {
  const key = string(value);
  return Object.hasOwn(labels, key) ? (labels[key] ?? '') : key;
}

function joined(values: readonly string[]): string {
  return values.filter(Boolean).join(' · ');
}

function rows(
  value: unknown,
  format: (entry: Readonly<Record<string, unknown>>) => string,
): string[] {
  return Array.isArray(value)
    ? value.map((entry: unknown) => format(object(entry))).filter(Boolean)
    : [];
}

function classification(values: Readonly<Record<string, unknown>>): string {
  const tags = rows(values.tags, (tag) =>
    tag.role === 'PRIMARY' || tag.role === 'SECONDARY' ? string(tag.name) : '',
  );
  return tags.join(' / ') || string(values.tag_cascade_id);
}

const fields = [
  ['name', '名称', (value) => [string(value)]],
  ['url', '网址', (value) => [string(value)]],
  ['summary', '简介', (value) => [string(value)]],
  ['blog_status', '标注状态', (value) => [label(value, statusLabels)]],
  ['access_scope', '访问范围', (value) => [label(value, scopeLabels)]],
  [
    'visibility',
    '可见状态',
    (value) => [label(value, { VISIBLE: '显示', HIDDEN: '隐藏', REMOVED: '已移除' })],
  ],
  ['tag_cascade_id', '分类路径', (_value, values) => [classification(values)]],
  [
    'tags',
    '标签',
    (value) =>
      rows(value, (tag) =>
        joined([string(tag.name) || string(tag.id), label(tag.role, tagRoleLabels)]),
      ),
  ],
  [
    'components',
    '站点组件',
    (value) =>
      rows(value, (component) =>
        joined([
          string(component.name) || string(component.id),
          label(component.role, componentRoleLabels),
        ]),
      ),
  ],
  [
    'feeds',
    '订阅源',
    (value) =>
      rows(value, (feed) =>
        joined([
          string(feed.name),
          string(feed.url),
          label(feed.format, feedFormatLabels),
          feed.is_enabled === false ? '停用' : '启用',
          feed.is_default === true ? '默认订阅源' : '',
        ]),
      ),
  ],
  [
    'resources',
    '资源',
    (value) =>
      rows(value, (resource) =>
        joined([label(resource.kind, resourceLabels), string(resource.url)]),
      ),
  ],
  [
    'outgoing',
    '本站的友链',
    (value) =>
      rows(value, (link) =>
        joined([string(link.target_url), label(link.link_status, linkStatusLabels)]),
      ),
  ],
  [
    'incoming',
    '指向本站的友链',
    (value) =>
      rows(value, (link) =>
        joined([
          string(link.source_name),
          string(link.source_url),
          label(link.link_status, linkStatusLabels),
        ]),
      ),
  ],
] satisfies readonly (readonly [
  string,
  string,
  (value: unknown, values: Readonly<Record<string, unknown>>) => string[],
])[];

export function annotationValues(values: Readonly<Record<string, unknown>>) {
  return fields.flatMap(([key, name, format]) =>
    Object.hasOwn(values, key)
      ? [{ name, entries: format(values[key], values).filter(Boolean) }]
      : [],
  );
}
