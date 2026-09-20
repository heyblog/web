import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { createRequire } from 'node:module';
import test from 'node:test';

import jsQR from 'jsqr';
import { PNG } from 'pngjs';

import { siteOgContent } from '../src/application/site-og/site-og.model.ts';
import { createSiteOgRenderer } from '../src/application/site-og/site-og.renderer.server.ts';
import { buildSiteOgSvg } from '../src/application/site-og/site-og.template.ts';

import { profile } from './site-og.fixture.ts';

test('renders real PNG bytes with packaged Chinese fonts and no external assets', async () => {
  // Given: the same WASM and fonts shipped with the application.
  const require = createRequire(import.meta.url);
  const renderer = await createSiteOgRenderer({
    wasm: new Uint8Array(await readFile(require.resolve('@resvg/resvg-wasm/index_bg.wasm'))),
    fonts: await Promise.all(
      ['Regular', 'Bold'].map((weight) =>
        readFile(new URL(`../src/assets/site-og/NotoSansSC-${weight}.otf`, import.meta.url)),
      ),
    ),
  });
  // When: two different site names render through the production renderer.
  const png = renderer(siteOgContent(profile));
  const different = renderer(siteOgContent({ ...profile, name: '另一个博客' }));
  // Then: images are valid, correctly sized, and contain site-specific pixels.
  assert.deepEqual([...png.slice(0, 8)], [137, 80, 78, 71, 13, 10, 26, 10]);
  const header = new DataView(png.buffer);
  assert.equal(header.getUint32(16), 1200);
  assert.equal(header.getUint32(20), 630);
  assert.notDeepEqual(png, different);
  const decoded = PNG.sync.read(Buffer.from(png));
  assert.equal(
    jsQR(new Uint8ClampedArray(decoded.data), decoded.width, decoded.height)?.data,
    'https://www.heyblog.net/site/38FycC0ow',
  );
  const half = new Uint8ClampedArray(600 * 315 * 4);
  for (let y = 0; y < 315; y += 1) {
    for (let x = 0; x < 600; x += 1) {
      const source = (y * 2 * decoded.width + x * 2) * 4;
      half.set(decoded.data.subarray(source, source + 4), (y * 600 + x) * 4);
    }
  }
  assert.equal(jsQR(half, 600, 315)?.data, 'https://www.heyblog.net/site/38FycC0ow');
  const icon = new PNG({ width: 32, height: 32 });
  for (let index = 0; index < icon.data.length; index += 4)
    icon.data.set([230, 80, 30, 255], index);
  const withIcon = renderer({
    ...siteOgContent(profile),
    iconDataUrl: `data:image/png;base64,${PNG.sync.write(icon).toString('base64')}`,
  });
  const iconPixels = PNG.sync.read(Buffer.from(withIcon)).data;
  let coloredPixels = 0;
  for (let index = 0; index < iconPixels.length; index += 4) {
    if (iconPixels[index] === 230 && iconPixels[index + 1] === 80 && iconPixels[index + 2] === 30)
      coloredPixels += 1;
  }
  assert.ok(coloredPixels > 100, 'The cached site icon must be visible in the rendered PNG');
});

test('escapes XML and independently truncates both classification levels', () => {
  // Given: untrusted content and two lengthy category labels.
  const content = {
    ...siteOgContent(profile),
    name: '<script>&"',
    description: '正文 < & >',
    host: 'example.com',
    classification: ['甲'.repeat(100), '乙'.repeat(100)] as const,
  };
  // When: content is interpolated into the image template.
  const svg = buildSiteOgSvg(content, (value, font) => Array.from(value).length * font.size);
  // Then: text remains text, and both category names retain visibility.
  assert.ok(svg.includes('&lt;script&gt;&amp;&quot;'));
  assert.ok(svg.includes('… · 乙'));
  assert.doesNotMatch(svg, /<script|<image|foreignObject/u);
  assert.ok(svg.includes('HeyBlog'));
});
