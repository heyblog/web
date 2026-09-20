import { initWasm, Resvg } from '@resvg/resvg-wasm';

import type { SiteOgContent } from './site-og.model.ts';
import { buildSiteOgSvg, type OgFont, ogStyle } from './site-og.template.ts';
import { escapeXml } from './site-og.text.ts';

export interface SiteOgAssets {
  readonly wasm: Uint8Array<ArrayBuffer>;
  readonly fonts: readonly Uint8Array[];
}

let ready: Promise<void> | undefined;

export async function createSiteOgRenderer(assets: SiteOgAssets) {
  ready ??= initWasm(assets.wasm).catch((error: unknown) => {
    ready = undefined;
    throw error;
  });
  await ready;
  const options = {
    font: { fontBuffers: [...assets.fonts], defaultFontFamily: ogStyle.fontFamily },
  };

  return (content: SiteOgContent): Uint8Array<ArrayBuffer> => {
    const widths = new Map<string, number>();
    const measure = (value: string, font: OgFont): number => {
      if (!value) return 0;
      const key = `${font.size}:${font.weight}:${value}`;
      const cached = widths.get(key);
      if (cached !== undefined) return cached;
      const svg = new Resvg(
        `<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="630"><text x="0" y="100" font-family="${ogStyle.fontFamily}" font-size="${font.size}" font-weight="${font.weight}">${escapeXml(value)}</text></svg>`,
        options,
      );
      try {
        const bounds = svg.innerBBox();
        const width = bounds ? bounds.width + Math.abs(bounds.x) : 0;
        bounds?.free();
        widths.set(key, width);
        return width;
      } finally {
        svg.free();
      }
    };
    const svg = new Resvg(buildSiteOgSvg(content, measure), options);
    try {
      const rendered = svg.render();
      try {
        return new Uint8Array(rendered.asPng());
      } finally {
        rendered.free();
      }
    } finally {
      svg.free();
    }
  };
}
