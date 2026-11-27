import type { EdgeRequest } from './protocol.ts';

export function publicIP(raw: string): boolean {
  const ip = raw.replace(/^\[|\]$/g, '').toLowerCase();
  if (ip.includes(':')) {
    // Accept only global-unicast space; exclude special-use/transition prefixes.
    if (!/^[23][0-9a-f]{3}:/u.test(ip) || ip.includes('.') || ip.includes('%')) return false;
    const [first, second = '0'] = ip.split(':');
    if (
      first === '2002' ||
      (first === '2001' && (Number.parseInt(second, 16) < 0x200 || second === 'db8'))
    )
      return false;
    return true;
  }
  const parts = ip.split('.');
  if (parts.length !== 4 || parts.some((part) => !/^\d{1,3}$/u.test(part) || Number(part) > 255))
    return false;
  const [a = 0, b = 0, c = 0] = parts.map(Number);
  return !(
    a === 0 ||
    a === 10 ||
    a === 127 ||
    a >= 224 ||
    (a === 100 && b >= 64 && b <= 127) ||
    (a === 169 && b === 254) ||
    (a === 172 && b >= 16 && b <= 31) ||
    (a === 192 && (b === 168 || (b === 0 && (c === 0 || c === 2)))) ||
    (a === 198 && (b === 18 || b === 19 || (b === 51 && c === 100))) ||
    (a === 203 && b === 0 && c === 113)
  );
}
export function safeTarget(request: EdgeRequest): boolean {
  const url = new URL(request.url);
  // Go signs the exact supplied URL. Fetch's WHATWG normalization must not
  // attach that proof to another path, host spelling, or effective URL.
  if (url.href !== request.url) return false;
  if (
    !['http:', 'https:'].includes(url.protocol) ||
    url.username ||
    url.password ||
    url.hash ||
    (url.port && !['80', '443'].includes(url.port)) ||
    !publicIP(request.resolvedIP)
  )
    return false;
  const host = url.hostname;
  if ((host.startsWith('[') || /^[\d.]+$/u.test(host)) && !publicIP(host)) return false;
  return (
    !['localhost', 'localhost.localdomain'].includes(host.toLowerCase()) &&
    !host.endsWith('.localhost')
  );
}
export function targetHeaders(
  source: EdgeRequest['headers'],
  encoding: EdgeRequest['headerEncoding'] = 'utf8',
): Headers | null {
  const headers = new Headers();
  for (const [name, value] of Object.entries(source)) {
    const key = name.toLowerCase();
    if (!key || /[^!#$%&'*+.^_`|~\da-z-]/u.test(key)) return null;
    for (const character of value) {
      const code = character.charCodeAt(0);
      if ((code < 32 && code !== 9) || code === 127 || (encoding === 'latin1' && code > 255))
        return null;
    }
    if (
      [
        'authorization',
        'proxy-authorization',
        'cookie',
        'host',
        'connection',
        'transfer-encoding',
        'content-length',
        'upgrade',
        'te',
        'trailer',
        'forwarded',
        'accept-encoding',
      ].includes(key) ||
      key.startsWith('cf-') ||
      key.startsWith('eo-') ||
      key.startsWith('x-forwarded-') ||
      (key.startsWith('x-heyblog-') && key !== 'x-heyblog-proof')
    )
      return null;
    // Go maps raw HTTP bytes to Latin-1 code points, preserving opaque validators.
    // Direct text callers use UTF-8; Fetch itself requires ByteString values.
    headers.set(
      name,
      encoding === 'latin1' ? value : String.fromCharCode(...new TextEncoder().encode(value)),
    );
  }
  headers.set('Cache-Control', 'no-cache, no-store');
  headers.set('Pragma', 'no-cache');
  return headers;
}
