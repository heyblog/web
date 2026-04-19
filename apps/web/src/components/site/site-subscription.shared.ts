const HTML_ENTITY_MAP: Record<string, string> = {
  '&nbsp;': ' ',
  '&amp;': '&',
  '&lt;': '<',
  '&gt;': '>',
  '&quot;': '"',
  '&#39;': "'",
  '&apos;': "'",
};

function decodeHtmlEntities(value: string): string {
  return value.replace(
    /&(nbsp|amp|lt|gt|quot|#39|apos);/gi,
    (matched) => HTML_ENTITY_MAP[matched.toLowerCase()] ?? matched,
  );
}

export function resolveSubscriptionIntro(value: string | null, maxLength = 140): string | null {
  const normalized = decodeHtmlEntities(value ?? '')
    .replace(/<[^>]*>/g, ' ')
    .replace(/\s+/g, ' ')
    .trim();

  if (!normalized) {
    return null;
  }

  if (normalized.length <= maxLength) {
    return normalized;
  }

  return `${normalized.slice(0, maxLength).trimEnd()}...`;
}
