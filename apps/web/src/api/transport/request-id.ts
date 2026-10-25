export function parseRequestID(value: string | null): string | null {
  return value && /^[A-Za-z0-9][A-Za-z0-9._-]{7,63}$/u.test(value) ? value : null;
}
