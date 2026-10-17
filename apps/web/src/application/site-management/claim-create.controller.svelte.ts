import {
  ClaimProblem,
  createClaimsTransport,
  hashClaimAttachment,
  uploadClaimAttachment,
} from '../../api/site-management/claims.browser.ts';
import type {
  ClaimAttachment,
  ClaimChallenge,
  ClaimContext,
  ClaimMethod,
} from '../../api/site-management/claims.types.ts';
import { createSiteAvailability } from '../site-submission/site-availability.controller.svelte.ts';
import { checkSiteAvailability } from '../site-submission/site-submission.api.browser.ts';

import {
  claimAttachmentContentType,
  claimAttachmentLimits,
  claimAttachmentTotal,
  manualClaimValidation,
  validateClaimAttachment,
} from './claim-attachments.ts';
import { claimCanCreate, claimMessage } from './claims.model.ts';

export function createClaimCreator(
  transport = createClaimsTransport(),
  availabilityRequest = checkSiteAvailability,
  uploadOrigin = '',
) {
  return new ClaimCreator(transport, availabilityRequest, uploadOrigin);
}
class ClaimCreator {
  context = $state<ClaimContext | null>(null);
  created = $state<ClaimChallenge | null>(null);
  attachments = $state<readonly ClaimAttachment[]>([]);
  attachmentProgress = $state(0);
  attachmentError = $state('');
  busy = $state(false);
  error = $state('');
  readonly availability: ReturnType<typeof createSiteAvailability>;
  private sequence = 0;
  private operation: AbortController | null = null;
  private readonly uploadOrigin: string;
  private readonly transport: ReturnType<typeof createClaimsTransport>;
  constructor(
    transport: ReturnType<typeof createClaimsTransport>,
    availabilityRequest: typeof checkSiteAvailability,
    uploadOrigin: string,
  ) {
    this.transport = transport;
    this.uploadOrigin = uploadOrigin;
    this.availability = createSiteAvailability(availabilityRequest);
  }
  clear() {
    this.sequence += 1;
    this.operation?.abort();
    this.availability.clear();
    this.context = null;
    this.attachments = [];
    this.attachmentProgress = 0;
    this.attachmentError = '';
    this.error = '';
    this.busy = false;
  }
  async addAttachments(files: readonly File[], address = this.context?.site.address ?? '') {
    const sequence = this.sequence;
    for (const file of files) {
      if (sequence !== this.sequence) return;
      await this.addAttachment(file, address);
      if (this.attachmentError) return;
    }
  }
  async addAttachment(file: File, address = this.context?.site.address ?? '') {
    if (!this.context || this.busy) return;
    const contentType = claimAttachmentContentType(file);
    const validation = validateClaimAttachment(
      file,
      this.attachments.length,
      claimAttachmentTotal(this.attachments),
    );
    if (validation || !contentType) {
      this.attachmentError = claimMessage(validation ?? 'manual_attachment_type');
      return;
    }
    const sequence = this.sequence;
    this.busy = true;
    this.attachmentError = '';
    this.attachmentProgress = 0;
    const operation = new AbortController();
    this.operation = operation;
    let attachmentID: string | undefined;
    try {
      const upload = await this.transport.initializeAttachment(
        {
          short_id: this.context.site.short_id,
          address,
          filename: file.name,
          content_type: contentType,
          size: file.size,
        },
        operation.signal,
      );
      attachmentID = upload.attachment.id;
      if (sequence !== this.sequence || operation.signal.aborted)
        throw new ClaimProblem('cancelled');
      if (!this.uploadOrigin || new URL(upload.upload_url).origin !== this.uploadOrigin)
        throw new ClaimProblem('invalid_response');
      if (file.size > upload.max_size) throw new ClaimProblem('manual_attachment_too_large');
      await uploadClaimAttachment(upload, file, operation.signal);
      const attachment = await this.transport.completeAttachment(
        attachmentID,
        {
          size: file.size,
          content_type: contentType,
          sha256: await hashClaimAttachment(file, operation.signal),
        },
        operation.signal,
      );
      if (sequence !== this.sequence || operation.signal.aborted)
        throw new ClaimProblem('cancelled');
      this.attachments = [...this.attachments, attachment];
      this.attachmentProgress = 1;
    } catch (error) {
      if (attachmentID) {
        try {
          await this.transport.removeAttachment(attachmentID, AbortSignal.timeout(10_000));
        } catch (cleanupError) {
          if (!(cleanupError instanceof ClaimProblem)) throw cleanupError;
        }
      }
      if (sequence !== this.sequence || operation.signal.aborted) return;
      if (error instanceof ClaimProblem) this.attachmentError = claimMessage(error.code);
      else throw error;
    } finally {
      if (sequence === this.sequence && this.operation === operation) {
        this.busy = false;
        this.operation = null;
      }
    }
  }
  async removeAttachment(id: string) {
    if (this.busy) return;
    const sequence = this.sequence;
    this.busy = true;
    this.attachmentError = '';
    const operation = new AbortController();
    this.operation = operation;
    try {
      await this.transport.removeAttachment(id, operation.signal);
      if (sequence !== this.sequence || operation.signal.aborted) return;
      this.attachments = this.attachments.filter((attachment) => attachment.id !== id);
    } catch (error) {
      if (sequence !== this.sequence || operation.signal.aborted) return;
      if (error instanceof ClaimProblem) this.attachmentError = claimMessage(error.code);
      else throw error;
    } finally {
      if (sequence === this.sequence && this.operation === operation) {
        this.busy = false;
        this.operation = null;
      }
    }
  }
  async select(id: string) {
    this.clear();
    const sequence = this.sequence;
    const operation = new AbortController();
    this.operation = operation;
    this.busy = true;
    try {
      const context = await this.transport.context(id, operation.signal);
      if (sequence === this.sequence) this.context = context;
    } catch (error) {
      if (sequence === this.sequence && error instanceof ClaimProblem)
        this.error = claimMessage(error.code);
      else if (!(error instanceof ClaimProblem)) throw error;
    } finally {
      if (sequence === this.sequence) this.busy = false;
    }
  }
  async create(
    input: {
      readonly method: ClaimMethod;
      readonly newAddress: boolean;
      readonly address: string;
      readonly evidence: string;
      readonly evidenceURL: string;
      readonly attachmentIDs?: readonly string[];
    },
    present = true,
  ) {
    if (
      !this.context ||
      this.busy ||
      this.availability.checking ||
      !claimCanCreate(this.context, input.newAddress)
    )
      return;
    const attachmentIDs =
      input.attachmentIDs ?? this.attachments.map((attachment) => attachment.id);
    if (input.method === 'MANUAL') {
      const validation = manualClaimValidation(input.evidence, input.evidenceURL, this.attachments);
      if (validation) {
        this.error = claimMessage(validation);
        return;
      }
      if (attachmentIDs.length > claimAttachmentLimits.maxCount) {
        this.error = claimMessage('manual_attachment_count');
        return;
      }
    }
    const siteShortID = this.context.site.short_id;
    const sequence = this.sequence;
    this.busy = true;
    this.error = '';
    const operation = new AbortController();
    this.operation = operation;
    try {
      if (
        input.newAddress &&
        !(await this.availability.check(
          { url: input.address, currentSiteShortID: siteShortID },
          true,
        ))
      )
        return;
      if (sequence !== this.sequence || operation.signal.aborted) return;
      const created = await this.transport.create(
        {
          short_id: siteShortID,
          method: input.method,
          ...(input.newAddress ? { address: input.address.trim() } : {}),
          ...(input.method === 'MANUAL'
            ? {
                evidence: input.evidence.trim(),
                evidence_url: input.evidenceURL.trim(),
                attachment_ids: attachmentIDs,
              }
            : {}),
        },
        operation.signal,
      );
      if (sequence !== this.sequence || operation.signal.aborted) return;
      if (present) this.created = created;
      return created;
    } catch (error) {
      if (sequence !== this.sequence || operation.signal.aborted) return;
      if (error instanceof ClaimProblem) {
        this.error = claimMessage(error.code);
        if (!operation.signal.aborted) {
          try {
            this.context = await this.transport.context(siteShortID, operation.signal);
          } catch (refreshError) {
            if (!(refreshError instanceof ClaimProblem)) throw refreshError;
          }
        }
      } else throw error;
    } finally {
      if (sequence === this.sequence) this.busy = false;
    }
  }
  dispose() {
    this.sequence += 1;
    this.operation?.abort();
    this.availability.dispose();
    this.created = null;
  }
}
