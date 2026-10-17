import type { ClaimAttachment } from '../../api/site-management/claims.types.ts';

export const claimAttachmentLimits = {
  maxCount: 3,
  maxTotalBytes: 20 * 1024 * 1024,
  maxImageBytes: 5 * 1024 * 1024,
  maxPdfBytes: 10 * 1024 * 1024,
} as const;
export type ClaimAttachmentValidationCode =
  | 'manual_attachment_count'
  | 'manual_attachment_type'
  | 'manual_attachment_too_large'
  | 'manual_attachment_total_too_large';
export function claimAttachmentContentType(file: Pick<File, 'name' | 'type'>): string | undefined {
  const filename = file.name.toLowerCase();
  if (filename.endsWith('.pdf'))
    return file.type && file.type !== 'application/pdf' ? undefined : 'application/pdf';
  if (filename.endsWith('.png'))
    return file.type && file.type !== 'image/png' ? undefined : 'image/png';
  if (filename.endsWith('.jpg') || filename.endsWith('.jpeg'))
    return file.type && file.type !== 'image/jpeg' ? undefined : 'image/jpeg';
  return undefined;
}
export function validateClaimAttachment(
  file: Pick<File, 'name' | 'type' | 'size'>,
  currentCount: number,
  currentTotalBytes: number,
): ClaimAttachmentValidationCode | undefined {
  if (currentCount >= claimAttachmentLimits.maxCount) return 'manual_attachment_count';
  const contentType = claimAttachmentContentType(file);
  if (!contentType) return 'manual_attachment_type';
  const perFileLimit =
    contentType === 'application/pdf'
      ? claimAttachmentLimits.maxPdfBytes
      : claimAttachmentLimits.maxImageBytes;
  if (file.size <= 0 || file.size > perFileLimit) return 'manual_attachment_too_large';
  if (currentTotalBytes + file.size > claimAttachmentLimits.maxTotalBytes)
    return 'manual_attachment_total_too_large';
  return undefined;
}
export function claimAttachmentTotal(
  attachments: readonly Pick<ClaimAttachment, 'size'>[],
): number {
  return attachments.reduce((total, attachment) => total + attachment.size, 0);
}
export function manualClaimValidation(
  evidence: string,
  evidenceURL: string,
  attachments: readonly ClaimAttachment[],
):
  | 'manual_evidence_too_short'
  | 'invalid_evidence'
  | 'invalid_evidence_url'
  | 'manual_attachment_required'
  | undefined {
  if (Array.from(evidence.trim()).length < 20) return 'manual_evidence_too_short';
  if (Array.from(evidence.trim()).length > 4000) return 'invalid_evidence';
  const url = evidenceURL.trim();
  if (url) {
    const parsed = URL.parse(url);
    if (
      !parsed ||
      parsed.username ||
      parsed.password ||
      (parsed.protocol !== 'http:' && parsed.protocol !== 'https:')
    )
      return 'invalid_evidence_url';
  }
  if (!url && attachments.length === 0) return 'manual_attachment_required';
  return undefined;
}
