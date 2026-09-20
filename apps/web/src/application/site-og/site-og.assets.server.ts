import wasmUrl from '@resvg/resvg-wasm/index_bg.wasm?url&inline';

import boldUrl from '../../assets/site-og/NotoSansSC-Bold.otf?url&inline';
import regularUrl from '../../assets/site-og/NotoSansSC-Regular.otf?url&inline';

import type { SiteOgContent } from './site-og.model.ts';
import { createSiteOgRenderer } from './site-og.renderer.server.ts';

const decode = (url: string) =>
  new Uint8Array(Buffer.from(url.slice(url.indexOf(',') + 1), 'base64'));
let renderer: ReturnType<typeof createSiteOgRenderer> | undefined;

export async function renderSiteOg(content: SiteOgContent): Promise<Uint8Array<ArrayBuffer>> {
  renderer ??= createSiteOgRenderer({
    wasm: decode(wasmUrl),
    fonts: [decode(regularUrl), decode(boldUrl)],
  }).catch((error: unknown) => {
    renderer = undefined;
    throw error;
  });
  return (await renderer)(content);
}
