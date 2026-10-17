const encoder = new TextEncoder();

export function utf8Length(value: string): number {
  return encoder.encode(value).length;
}

export function characterLength(value: string): number {
  return Array.from(value).length;
}
