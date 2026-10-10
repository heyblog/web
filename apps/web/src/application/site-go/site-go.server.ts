import { loadSiteDirectoryOptions } from '../../api/sites/site-directory.server.ts';
import { loadRandomSite } from '../../api/sites/site-random.server.ts';
import type { ApiJsonDependencies } from '../../api/transport/client.server.ts';

import { buildSiteGoHref, parseSiteGoQuery, siteGoParameterMessage } from './site-go.shared.ts';

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
    loadSiteDirectoryOptions(request, dependencies),
  ]);
  const classifications = options.kind === 'success' ? options.data.classifications : [];
  const requested = parsed.kind === 'valid' ? parsed.query : { level1: '', level2: '' };
  const parent = classifications.find(
    (item) => item.label.toLowerCase() === requested.level1.toLowerCase(),
  );
  const level1 = parent?.label ?? '';
  const level2 =
    parent?.children.find((item) => item.label.toLowerCase() === requested.level2.toLowerCase())
      ?.label ?? '';
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
