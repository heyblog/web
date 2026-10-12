import { ClaimProblem, createClaimsTransport } from '../../api/site-management/claims.browser.ts';
import type {
  ClaimChallenge,
  ClaimContext,
  ClaimMethod,
} from '../../api/site-management/claims.types.ts';

import { claimCanCreate, claimMessage } from './claims.model.ts';

export function createClaimCreator(transport = createClaimsTransport()) {
  return new ClaimCreator(transport);
}
class ClaimCreator {
  context = $state<ClaimContext | null>(null);
  created = $state<ClaimChallenge | null>(null);
  busy = $state(false);
  error = $state('');
  private sequence = 0;
  private operation: AbortController | null = null;
  private readonly transport: ReturnType<typeof createClaimsTransport>;
  constructor(transport: ReturnType<typeof createClaimsTransport>) {
    this.transport = transport;
  }
  clear() {
    this.sequence += 1;
    this.operation?.abort();
    this.context = null;
    this.error = '';
    this.busy = false;
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
  async create(input: {
    readonly method: ClaimMethod;
    readonly newAddress: boolean;
    readonly address: string;
    readonly evidence: string;
    readonly evidenceURL: string;
  }) {
    if (!this.context || this.busy || !claimCanCreate(this.context, input.newAddress)) return;
    this.busy = true;
    this.error = '';
    const operation = new AbortController();
    this.operation = operation;
    try {
      this.created = await this.transport.create(
        {
          short_id: this.context.site.short_id,
          method: input.method,
          ...(input.newAddress ? { address: input.address.trim() } : {}),
          ...(input.method === 'MANUAL'
            ? { evidence: input.evidence.trim(), evidence_url: input.evidenceURL.trim() }
            : {}),
        },
        operation.signal,
      );
    } catch (error) {
      if (error instanceof ClaimProblem) {
        this.error = claimMessage(error.code);
        if (!operation.signal.aborted) {
          try {
            this.context = await this.transport.context(
              this.context.site.short_id,
              operation.signal,
            );
          } catch (refreshError) {
            if (!(refreshError instanceof ClaimProblem)) throw refreshError;
          }
        }
      } else throw error;
    } finally {
      this.busy = false;
    }
  }
  dispose() {
    this.sequence += 1;
    this.operation?.abort();
    this.created = null;
  }
}
