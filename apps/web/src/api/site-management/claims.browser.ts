import { parseEvidenceDownload } from './claim-attachment.types.ts';
import {
  type ClaimMethod,
  parseChallenge,
  parseClaim,
  parseClaimAttachment,
  parseClaimAttachmentUpload,
  parseClaimContext,
  parseOwnership,
  record,
} from './claims.types.ts';
import { accountRequest } from './site-management.browser.ts';

export class ClaimProblem extends Error {
  readonly code: string;
  constructor(code: string) {
    super(code);
    this.name = 'ClaimProblem';
    this.code = code;
  }
}
export function createClaimsTransport(request = accountRequest) {
  async function send<T>(
    path: string,
    parser: (value: unknown) => T,
    options: Parameters<typeof accountRequest>[1] = {},
  ): Promise<T> {
    let response: Response;
    try {
      response = await request(path, options);
    } catch (error) {
      if (error instanceof Error)
        throw new ClaimProblem(options.signal?.aborted ? 'cancelled' : 'network_error');
      throw error;
    }
    if (!response.ok) {
      let code =
        response.status === 401
          ? 'unauthorized'
          : response.status === 429
            ? 'rate_limited'
            : 'request_failed';
      try {
        const problem = record(await response.json());
        if (typeof problem.code === 'string') code = problem.code;
      } catch (error) {
        if (!(error instanceof Error)) throw error;
      }
      throw new ClaimProblem(code);
    }
    try {
      return parser(await response.json());
    } catch (error) {
      if (error instanceof Error) throw new ClaimProblem('invalid_response');
      throw error;
    }
  }
  return {
    context: (id: string, signal: AbortSignal) =>
      send(`sites/${encodeURIComponent(id)}/claim-context`, parseClaimContext, { signal }),
    detail: (id: string, signal: AbortSignal, management = false) =>
      send(`site-claims/${encodeURIComponent(id)}`, parseClaim, { signal, management }),
    create: (
      body: {
        readonly short_id: string;
        readonly method: ClaimMethod;
        readonly address?: string;
        readonly evidence?: string;
        readonly evidence_url?: string;
        readonly attachment_ids?: readonly string[];
      },
      signal: AbortSignal,
    ) => send('site-claims', parseChallenge, { method: 'POST', body, signal }),
    initializeAttachment: (
      body: {
        readonly short_id: string;
        readonly address: string;
        readonly filename: string;
        readonly content_type: string;
        readonly size: number;
      },
      signal: AbortSignal,
    ) =>
      send('site-claim-attachments', parseClaimAttachmentUpload, { method: 'POST', body, signal }),
    completeAttachment: (
      id: string,
      body: { readonly size: number; readonly content_type: string; readonly sha256: string },
      signal: AbortSignal,
    ) =>
      send(
        `site-claim-attachments/${encodeURIComponent(id)}/complete`,
        (value) => {
          const item = record(value);
          return parseClaimAttachment(item.attachment ?? value);
        },
        { method: 'POST', body, signal },
      ),
    downloadAttachment: (id: string, signal: AbortSignal, management = false) =>
      send(`site-claim-attachments/${encodeURIComponent(id)}/download`, parseEvidenceDownload, {
        method: 'POST',
        signal,
        management,
      }),
    removeAttachment: (id: string, signal: AbortSignal) =>
      send(
        `site-claim-attachments/${encodeURIComponent(id)}`,
        (value) => {
          const item = record(value);
          if (item.success !== true) throw new TypeError('Invalid attachment deletion result');
          return true;
        },
        { method: 'DELETE', signal },
      ),
    change: (id: string, operation: 'check' | 'cancel', signal: AbortSignal) =>
      send(
        `site-claims/${encodeURIComponent(id)}${operation === 'check' ? '/check' : ''}`,
        parseClaim,
        { method: operation === 'check' ? 'POST' : 'DELETE', signal },
      ),
    regenerate: (id: string, signal: AbortSignal) =>
      send(`site-claims/${encodeURIComponent(id)}/regenerate`, parseChallenge, {
        method: 'POST',
        signal,
      }),
    review: (
      id: string,
      body: { readonly approve: boolean; readonly reason: string },
      signal: AbortSignal,
    ) =>
      send(`site-claims/${encodeURIComponent(id)}/review`, parseClaim, {
        method: 'POST',
        body,
        management: true,
        signal,
      }),
    ownership: (id: string, signal: AbortSignal) =>
      send(`site-ownership/${encodeURIComponent(id)}`, parseOwnership, {
        management: true,
        signal,
      }),
    revoke: (
      id: string,
      body: {
        readonly ownership_id: string;
        readonly revision: number;
        readonly reason: string;
        readonly evidence: string;
      },
      signal: AbortSignal,
    ) =>
      send(
        `site-ownership/${encodeURIComponent(id)}`,
        (value) => {
          const result = record(value);
          if (result.success !== true) throw new TypeError('Invalid revocation result');
          return true;
        },
        { method: 'DELETE', body, management: true, signal },
      ),
  };
}

export async function uploadClaimAttachment(
  upload: {
    readonly upload_url: string;
    readonly upload_headers: Readonly<Record<string, string>>;
    readonly content_type: string;
  },
  file: Blob,
  signal: AbortSignal,
  fetcher: typeof globalThis.fetch = globalThis.fetch,
): Promise<void> {
  const headers = new Headers(upload.upload_headers);
  if (!headers.has('Content-Type')) headers.set('Content-Type', upload.content_type);
  let response: Response;
  try {
    response = await fetcher(upload.upload_url, {
      method: 'PUT',
      body: file,
      headers,
      credentials: 'omit',
      cache: 'no-store',
      signal: AbortSignal.any([AbortSignal.timeout(120_000), signal]),
    });
  } catch (error) {
    if (error instanceof Error)
      throw new ClaimProblem(signal.aborted ? 'cancelled' : 'attachment_upload_failed');
    throw error;
  }
  if (!response.ok) throw new ClaimProblem('attachment_upload_failed');
}

export async function hashClaimAttachment(file: Blob, signal: AbortSignal): Promise<string> {
  if (!globalThis.crypto?.subtle) throw new ClaimProblem('attachment_hash_failed');
  if (signal.aborted) throw new ClaimProblem('cancelled');
  let digest: ArrayBuffer;
  try {
    digest = await globalThis.crypto.subtle.digest('SHA-256', await file.arrayBuffer());
  } catch (error) {
    if (error instanceof Error) throw new ClaimProblem('attachment_hash_failed');
    throw error;
  }
  return Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, '0')).join('');
}
