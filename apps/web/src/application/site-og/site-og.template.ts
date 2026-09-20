import type { SiteOgContent } from './site-og.model.ts';
import { renderSiteQr } from './site-og.qr.server.ts';
import { escapeXml, fitText, type MeasureText, wrapText } from './site-og.text.ts';

// The reference palette is retained; the site's identity now owns the main area.
export const ogStyle = {
  width: 1200,
  height: 630,
  columnX: 88,
  columnWidth: 768,
  fontFamily: 'Noto Sans SC',
  colors: {
    outer: '#D9EEEF',
    surface: '#F4F4F5',
    rule: '#D4D4D8',
    brand: '#158187',
    text: '#241E18',
    muted: '#6A5B4E',
    white: '#FFFFFF',
    badge: '#0C666B',
  },
  title: { size: 56, weight: 700 },
  summary: { size: 28, weight: 400 },
  domain: { size: 26, weight: 400 },
  category: { size: 22, weight: 400 },
} as const;

export interface OgFont {
  readonly size: number;
  readonly weight: number;
}
export type OgMeasure = (value: string, font: OgFont) => number;

export function buildSiteOgSvg(content: SiteOgContent, measure: OgMeasure): string {
  const { colors, columnX: x, columnWidth: width } = ogStyle;
  const withFont =
    (font: OgFont): MeasureText =>
    (value) =>
      measure(value, font);
  const title = wrapText(content.name, {
    width: width - 96,
    maxLines: 2,
    measure: withFont(ogStyle.title),
  });
  const summary = wrapText(content.description, {
    width,
    maxLines: 3,
    measure: withFont(ogStyle.summary),
  });
  const domain = fitText(content.host, width - 96, withFont(ogStyle.domain));
  const titleHeight = title.length * 68;
  const categoryHeight = content.classification ? 64 : 0;
  const totalHeight = titleHeight + 14 + 34 + 28 + categoryHeight + summary.length * 40;
  const top = Math.round(80 + (408 - totalHeight) / 2);
  const text = (value: string, font: OgFont, location: { x: number; y: number; color: string }) =>
    `<text x="${location.x}" y="${location.y}" fill="${location.color}" font-size="${font.size}" font-weight="${font.weight}">${escapeXml(value)}</text>`;
  const domainY = top + titleHeight + 32;
  const categoryY = top + titleHeight + 76;
  const summaryY = categoryY + categoryHeight + 28;
  let category = '';
  if (content.classification) {
    const measureCategory = withFont(ogStyle.category);
    const budget = width - 32 - measureCategory(' · ');
    const [first, second] = content.classification;
    const firstWidth = Math.min(
      measureCategory(first),
      Math.max(budget / 2, budget - measureCategory(second)),
    );
    const label = `${fitText(first, firstWidth, measureCategory)} · ${fitText(second, budget - firstWidth, measureCategory)}`;
    category = `<rect x="${x}" y="${categoryY}" width="${measureCategory(label) + 32}" height="40" rx="6" fill="${colors.outer}"/>
      ${text(label, ogStyle.category, { x: x + 16, y: categoryY + 28, color: colors.badge })}`;
  }
  const initial =
    new Intl.Segmenter('zh-CN', { granularity: 'grapheme' })
      .segment(content.name)
      [Symbol.iterator]()
      .next().value?.segment ?? 'H';
  const siteIcon = content.iconDataUrl
    ? `<image x="${x + 4}" y="${top + 4}" width="64" height="64" preserveAspectRatio="xMidYMid meet" href="${escapeXml(content.iconDataUrl)}"/>`
    : `<text x="${x + 36}" y="${top + 49}" text-anchor="middle" fill="${colors.badge}" font-size="36" font-weight="700">${escapeXml(initial)}</text>`;
  return `<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="630" viewBox="0 0 1200 630" fill="none" font-family="${ogStyle.fontFamily}">
    <rect width="1200" height="630" rx="32" fill="${colors.outer}"/>
    <rect x="24" y="24" width="1152" height="582" rx="24" fill="${colors.surface}" stroke="${colors.rule}" stroke-width="2"/>
    <rect x="${x}" y="${top}" width="72" height="72" rx="12" fill="${colors.outer}"/>
    ${siteIcon}
    ${title.map((line, index) => text(line, ogStyle.title, { x: x + 96, y: top + 54 + index * 68, color: colors.text })).join('')}
    ${text(domain, ogStyle.domain, { x: x + 96, y: domainY, color: colors.muted })}
    ${category}
    ${summary.map((line, index) => text(line, ogStyle.summary, { x, y: summaryY + index * 40, color: colors.muted })).join('')}
    ${renderSiteQr(content.detailUrl)}
    <text x="1020" y="460" text-anchor="middle" fill="${colors.muted}" font-size="22">站点详情</text>
    <path d="M88 526H1112" stroke="${colors.rule}" stroke-width="2"/>
    <svg x="88" y="547" width="40" height="40" viewBox="0 0 256 256">
      <rect width="256" height="256" rx="40" fill="${colors.brand}"/>
      <g transform="translate(197 56) scale(-6 6)" stroke="${colors.white}" stroke-width="4" stroke-linecap="round" stroke-linejoin="round" vector-effect="non-scaling-stroke">
        <path d="M9 6h11M12 12h8M15 18h5M3 6v.01M6 12v.01M9 18v.01"/>
      </g>
    </svg>
    <text x="136" y="574" fill="${colors.muted}" font-size="24" font-weight="700">HeyBlog</text>
  </svg>`;
}
