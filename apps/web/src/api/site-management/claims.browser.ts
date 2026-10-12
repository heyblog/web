import {
  type ClaimMethod,
  parseChallenge,
  parseClaim,
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
      },
      signal: AbortSignal,
    ) => send('site-claims', parseChallenge, { method: 'POST', body, signal }),
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
