import {
  type ApiJsonDependencies,
  type ApiJsonResult,
  fetchApiJson,
} from '../api/client.server.ts';
import type { SiteCardView } from '../home/home.shared.ts';
import type { SiteDirectoryOptions } from '../site-directory/site-directory.models.ts';

import { buildSiteGoHref, parseSiteGoQuery, siteGoParameterMessage } from './site-go.shared.ts';

export interface RandomSiteView {
  site: SiteCardView | null;
}

export function loadRandomSite(
  search: string,
  request?: Request,
  dependencies: ApiJsonDependencies = {},
): Promise<ApiJsonResult<RandomSiteView>> {
  return fetchApiJson<RandomSiteView>(`/sites/random${search}`, {
    ...dependencies,
    request,
    signal: request?.signal,
  });
}

export async function loadSiteGoPage(
  url: URL,
  request?: Request,
  dependencies: ApiJsonDependencies = {},
) {
  const parsed = parseSiteGoQuery(url.searchParams);
  const preview = parsed.kind === 'valid' ? parsed.query.preview : parsed.preview;
  const selectionPromise =
    parsed.kind === 'valid'
      ? loadRandomSite(
          new URL(buildSiteGoHref(parsed.query.level1, parsed.query.level2), url).search,
          request,
          dependencies,
        )
      : Promise.resolve({ kind: 'bad-request' as const, code: parsed.code });
  const [selection, options] = await Promise.all([
    selectionPromise,
    fetchApiJson<SiteDirectoryOptions>('/sites/options', {
      ...dependencies,
      request,
      signal: request?.signal,
    }),
  ]);
  const classifications = options.kind === 'success' ? options.data.classifications : [];
  const requested = parsed.kind === 'valid' ? parsed.query : { level1: '', level2: '' };
  const parent = classifications.find((item) => item.label === requested.level1);
  const level1 = parent?.label ?? '';
  const level2 = parent?.children.find((item) => item.label === requested.level2)?.label ?? '';
  const invalid = selection.kind === 'bad-request';
  const unavailable =
    !invalid && (selection.kind !== 'success' || (preview && options.kind !== 'success'));
  const site = selection.kind === 'success' ? selection.data.site : null;
  const message = invalid
    ? siteGoParameterMessage(selection.code)
    : unavailable
      ? '暂时无法获取博客或分类，请稍后重试。'
      : '当前分类下还没有公开博客，可以扩大分类范围后再试。';
  return {
    preview,
    classifications,
    level1,
    level2,
    site,
    invalid,
    unavailable,
    message,
    status: invalid ? 400 : unavailable ? 503 : 200,
    // Keep validated filters even if optional classification metadata is unavailable.
    rerollHref: buildSiteGoHref(requested.level1, requested.level2, preview),
    editorHref: buildSiteGoHref(
      invalid ? '' : requested.level1,
      invalid ? '' : requested.level2,
      true,
    ),
    recoveryHref: buildSiteGoHref('', '', preview),
  };
}
