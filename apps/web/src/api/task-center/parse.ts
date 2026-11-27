export class ProtocolError extends Error {}
function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}
export function record(value: unknown): Record<string, unknown> {
  if (!isRecord(value)) throw new ProtocolError('invalid_response');
  return value;
}
export function text(value: unknown): string {
  if (typeof value !== 'string') throw new ProtocolError('invalid_response');
  return value;
}
export function number(value: unknown): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value))
    throw new ProtocolError('invalid_response');
  return value;
}
export function bool(value: unknown): boolean {
  if (typeof value !== 'boolean') throw new ProtocolError('invalid_response');
  return value;
}
export function nullable(value: unknown): string | null {
  return value === null ? null : text(value);
}
export function list<T>(value: unknown, parser: (item: unknown) => T): T[] {
  if (!Array.isArray(value)) throw new ProtocolError('invalid_response');
  return value.map(parser);
}
export function strings(value: unknown): Record<string, string> {
  return Object.fromEntries(Object.entries(record(value)).map(([key, item]) => [key, text(item)]));
}
export function choice<const T extends readonly string[]>(value: unknown, choices: T): T[number] {
  const found = choices.find((item) => item === value);
  if (!found) throw new ProtocolError('invalid_response');
  return found;
}
