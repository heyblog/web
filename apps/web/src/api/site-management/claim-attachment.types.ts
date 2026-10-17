export interface ClaimAttachment {
  readonly id: string;
  readonly filename: string;
  readonly content_type: string;
  readonly size: number;
  readonly status: string;
}
export interface ClaimAttachmentUpload {
  readonly attachment: ClaimAttachment;
  readonly upload_url: string;
  readonly upload_headers: Readonly<Record<string, string>>;
  readonly expires_at: string;
  readonly max_size: number;
  readonly content_type: string;
}
function record(value: unknown): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value))
    throw new TypeError('Invalid claim attachment response');
  return Object.fromEntries(Object.entries(value));
}
function text(value: unknown): string {
  if (typeof value !== 'string') throw new TypeError('Invalid claim attachment text');
  return value;
}
function positiveInteger(value: unknown): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 0)
    throw new TypeError('Invalid claim attachment size');
  return value;
}
function headers(value: unknown): Readonly<Record<string, string>> {
  if (value == null) return {};
  const item = record(value);
  return Object.fromEntries(Object.entries(item).map(([key, entry]) => [key, text(entry)]));
}
export function parseClaimAttachment(value: unknown): ClaimAttachment {
  const item = record(value);
  return {
    id: text(item.id),
    filename: text(item.filename),
    content_type: text(item.content_type),
    size: positiveInteger(item.size),
    status: text(item.status),
  };
}
export function parseClaimAttachments(value: unknown): readonly ClaimAttachment[] {
  if (!Array.isArray(value)) throw new TypeError('Invalid claim attachments');
  return value.map(parseClaimAttachment);
}
export function parseClaimAttachmentUpload(value: unknown): ClaimAttachmentUpload {
  const item = record(value);
  const url = text(item.upload_url);
  const parsed = URL.parse(url);
  if (!parsed || (parsed.protocol !== 'http:' && parsed.protocol !== 'https:'))
    throw new TypeError('Invalid claim attachment upload URL');
  const expiresAt = text(item.expires_at);
  if (!Number.isFinite(Date.parse(expiresAt))) throw new TypeError('Invalid claim attachment date');
  return {
    attachment: parseClaimAttachment(item.attachment),
    upload_url: url,
    upload_headers: headers(item.upload_headers),
    expires_at: expiresAt,
    max_size: positiveInteger(item.max_size),
    content_type: text(item.content_type),
  };
}

export function parseEvidenceOrigin(value: unknown): string {
  const origin = text(record(value).upload_origin);
  if (origin === '') return '';
  const url = new URL(origin);
  if (url.origin !== origin || !isEvidenceURL(url))
    throw new TypeError('Invalid evidence storage origin');
  return origin;
}
function isEvidenceURL(url: URL): boolean {
  return (
    url.protocol === 'https:' &&
    !url.username &&
    !url.password &&
    !url.port &&
    /^[a-z0-9-]+\.cos\.[a-z0-9-]+\.myqcloud\.com$/u.test(url.hostname)
  );
}
export function parseEvidenceDownload(value: unknown): string {
  const url = new URL(text(record(value).url));
  if (!isEvidenceURL(url)) throw new TypeError('Invalid evidence download URL');
  return url.href;
}
