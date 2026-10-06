import type { AuditAction, SubmissionPayload } from './site-submission.types.ts';

export interface BrowserRequestOptions {
  readonly signal?: AbortSignal;
  readonly fetch?: typeof globalThis.fetch;
}

export interface SubmissionProblem {
  readonly code: string;
  readonly slugCandidates: readonly string[];
  readonly slugConflicts: readonly string[];
}

export async function readSubmissionProblem(response: Response): Promise<SubmissionProblem> {
  const payload: unknown = await response.json().catch(() => null);
  const result: SubmissionProblem = {
    code: 'request_failed',
    slugCandidates: [],
    slugConflicts: [],
  };
  if (!payload || typeof payload !== 'object' || !('code' in payload)) return result;
  const code = typeof payload.code === 'string' ? payload.code : result.code;
  if (
    code !== 'slug_needs_confirmation' ||
    !('invalid_params' in payload) ||
    !Array.isArray(payload.invalid_params)
  )
    return { ...result, code };

  const slugCandidates: string[] = [];
  const slugConflicts: string[] = [];
  const params: readonly unknown[] = payload.invalid_params;
  for (const param of params.slice(0, 21)) {
    if (
      !param ||
      typeof param !== 'object' ||
      !('name' in param) ||
      typeof param.name !== 'string' ||
      !('reason' in param) ||
      typeof param.reason !== 'string'
    )
      continue;
    if (
      /^tags\..+\.slug$/.test(param.name) &&
      /^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(param.reason) &&
      param.reason.length <= 160
    ) {
      slugCandidates.push(param.reason);
    } else if (
      /^conflicts\.(?:[0-9a-f-]{36})?$/.test(param.name) &&
      /^.{1,120} \([a-z0-9]+(?:-[a-z0-9]+)*\)$/.test(param.reason) &&
      param.reason.length <= 283
    ) {
      slugConflicts.push(param.reason);
    }
  }
  return { code, slugCandidates, slugConflicts };
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
