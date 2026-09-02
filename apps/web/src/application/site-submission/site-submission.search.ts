function compact(value: string): string {
  return value.toLocaleLowerCase('zh-CN').replaceAll(/[^\p{L}\p{N}]+/gu, '');
}
function isSubsequence(needle: string, haystack: string): boolean {
  let index = 0;
  for (const character of haystack) {
    if (character === needle[index]) index += 1;
    if (index === needle.length) return true;
  }
  return needle.length === 0;
}
export function matchesSubmissionOption(query: string, label: string): boolean {
  const normalizedLabel = compact(label);
  const tokens = query
    .toLocaleLowerCase('zh-CN')
    .split(/[^\p{L}\p{N}]+/u)
    .map(compact)
    .filter(Boolean);
  return tokens.every(
    (token) => normalizedLabel.includes(token) || isSubsequence(token, normalizedLabel),
  );
}
