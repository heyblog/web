export type MeasureText = (value: string) => number;

const segmenter = new Intl.Segmenter('zh-CN', { granularity: 'grapheme' });
const graphemes = (value: string) => Array.from(segmenter.segment(value), ({ segment }) => segment);

function fittingPrefix(
  parts: readonly string[],
  width: number,
  measure: MeasureText,
  suffix = '',
): number {
  let low = 0;
  let high = parts.length;
  while (low < high) {
    const middle = Math.ceil((low + high) / 2);
    if (measure(parts.slice(0, middle).join('') + suffix) <= width) low = middle;
    else high = middle - 1;
  }
  return low;
}

export function fitText(value: string, width: number, measure: MeasureText): string {
  if (measure(value) <= width) return value;
  const parts = graphemes(value);
  return `${parts
    .slice(0, fittingPrefix(parts, width, measure, '…'))
    .join('')
    .trimEnd()}…`;
}

export function wrapText(
  value: string,
  options: {
    readonly width: number;
    readonly maxLines: number;
    readonly measure: MeasureText;
  },
): readonly string[] {
  let remaining = value.replace(/\s+/gu, ' ').trim();
  const lines: string[] = [];
  while (remaining && lines.length < options.maxLines) {
    if (lines.length === options.maxLines - 1 || options.measure(remaining) <= options.width) {
      lines.push(fitText(remaining, options.width, options.measure));
      break;
    }
    const parts = graphemes(remaining);
    let count = fittingPrefix(parts, options.width, options.measure);
    if (count === 0) {
      lines.push('…');
      break;
    }
    // Prefer Latin word boundaries without wasting more than a quarter of the line.
    const prefix = parts.slice(0, count).join('');
    const space = prefix.lastIndexOf(' ');
    if (space >= prefix.length * 0.75) count = graphemes(prefix.slice(0, space)).length;
    lines.push(parts.slice(0, count).join('').trimEnd());
    remaining = parts.slice(count).join('').trimStart();
  }
  return lines;
}

export function escapeXml(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&apos;')
    .split('')
    .filter((character) => character >= ' ' || '\t\n\r'.includes(character))
    .join('');
}
