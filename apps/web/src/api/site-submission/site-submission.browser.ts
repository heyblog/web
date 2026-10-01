import type { AuditAction, SubmissionPayload } from './site-submission.types.ts';

export interface BrowserRequestOptions {
  readonly signal?: AbortSignal;
  readonly fetch?: typeof globalThis.fetch;
}

interface ProblemPayload {
  readonly code?: unknown;
}

export async function readSubmissionProblemCode(response: Response): Promise<string> {
  const payload = (await response.json().catch(() => null)) as ProblemPayload | null;
  return typeof payload?.code === 'string' ? payload.code : 'request_failed';
}

export function submissionEndpoint(action: AuditAction, siteShortID: string): string {
  if (action === 'CREATE') return '/api/site-submissions/create';
  const suffix = action === 'UPDATE' ? 'update' : action === 'DELETE' ? 'delete' : 'restore';
  return '/api/site-submissions/' + encodeURIComponent(siteShortID) + '/' + suffix;
}

export interface SubmissionRequest {
  readonly action: AuditAction;
  readonly siteShortID: string;
  readonly payload: SubmissionPayload;
}

export function submitSite(
  submission: SubmissionRequest,
  options: BrowserRequestOptions = {},
): Promise<Response> {
  const request = options.fetch ?? globalThis.fetch;
  return request(submissionEndpoint(submission.action, submission.siteShortID), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(submission.payload),
    signal: options.signal,
  });
}

export function requestSiteAvailability(
  url: string,
  options: BrowserRequestOptions = {},
): Promise<Response> {
  const request = options.fetch ?? globalThis.fetch;
  const endpoint = '/api/site-submissions/availability?url=' + encodeURIComponent(url);
  return request(endpoint, { signal: options.signal });
}

export function requestSubmissionOptions(): Promise<Response> {
  return fetch('/api/site-submissions/options');
}

export function requestSubmissionSnapshot(siteShortID: string): Promise<Response> {
  return fetch(`/api/site-submissions/${encodeURIComponent(siteShortID)}/resolve`);
}

export function requestSubmissionSearch(value: string, signal: AbortSignal): Promise<Response> {
  return fetch(`/api/site-submissions/sites?q=${encodeURIComponent(value)}`, { signal });
}

export function requestAuditLookup(lookupToken: string): Promise<Response> {
  return fetch('/api/site-submissions/query', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ lookup_token: lookupToken }),
  });
}

export interface ReviewExpectation {
  readonly expected_site_revision: number;
  readonly expected_review_draft_revision: number;
}

export interface ReviewDecision extends ReviewExpectation {
  readonly decision: 'APPROVED' | 'REJECTED';
  readonly reviewer_comment: string;
}

export interface ReviewDraft extends ReviewExpectation {
  readonly site: SubmissionPayload['site'];
}

export function requestAuditReview(
  auditID: string,
  discard: boolean,
  payload: ReviewExpectation | ReviewDecision,
): Promise<Response> {
  return fetch(`/management/site-submissions/${auditID}/${discard ? 'review-draft' : 'review'}`, {
    method: discard ? 'DELETE' : 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
}

export function requestReviewDraft(auditID: string, payload: ReviewDraft): Promise<Response> {
  return fetch(`/management/site-submissions/${auditID}/review-draft`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
}
