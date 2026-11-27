export function record(value: unknown): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value))
    throw new TypeError('Invalid annotation object');
  return Object.fromEntries(Object.entries(value));
}

export function text(value: unknown): string {
  if (typeof value !== 'string') throw new TypeError('Invalid annotation text');
  return value;
}

export function int(value: unknown, minimum = 0): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < minimum)
    throw new TypeError('Invalid annotation number');
  return value;
}

export function bool(value: unknown): boolean {
  if (typeof value !== 'boolean') throw new TypeError('Invalid annotation flag');
  return value;
}

export function oneOf<T extends string>(value: unknown, options: readonly T[]): T {
  const result = options.find((item) => item === value);
  if (result === undefined) throw new TypeError('Invalid annotation option');
  return result;
}

export function list<T>(value: unknown, parse: (entry: unknown) => T): T[] {
  if (!Array.isArray(value)) throw new TypeError('Invalid annotation list');
  return value.map(parse);
}

export function timestamp(value: unknown): string {
  const date = text(value);
  if (!Number.isFinite(Date.parse(date))) throw new TypeError('Invalid annotation date');
  return date;
}
