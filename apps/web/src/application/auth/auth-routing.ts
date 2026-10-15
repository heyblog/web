export function safeNext(value: FormDataEntryValue | string | null | undefined): string {
  if (typeof value !== 'string') return '/dashboard';
  const path = value.trim();
  if (
    !path.startsWith('/') ||
    path.startsWith('//') ||
    path.includes('\\') ||
    [...path].some((character) => character.charCodeAt(0) < 32 || character.charCodeAt(0) === 127)
  )
    return '/dashboard';
  const target = new URL(path, 'https://local.invalid');
  return target.origin === 'https://local.invalid'
    ? target.pathname + target.search + target.hash
    : '/dashboard';
}
