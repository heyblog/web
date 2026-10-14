import { ClaimProblem, createClaimsTransport } from '../../api/site-management/claims.browser.ts';
import type {
  ClaimChallenge,
  ClaimContext,
  ClaimMethod,
} from '../../api/site-management/claims.types.ts';
import { createSiteAvailability } from '../site-submission/site-availability.controller.svelte.ts';
import { checkSiteAvailability } from '../site-submission/site-submission.api.browser.ts';

import { claimCanCreate, claimMessage } from './claims.model.ts';

export function createClaimCreator(
  transport = createClaimsTransport(),
  availabilityRequest = checkSiteAvailability,
) {
  return new ClaimCreator(transport, availabilityRequest);
}
class ClaimCreator {
  context = $state<ClaimContext | null>(null);
  created = $state<ClaimChallenge | null>(null);
  busy = $state(false);
  error = $state('');
  readonly availability: ReturnType<typeof createSiteAvailability>;
  private sequence = 0;
  private operation: AbortController | null = null;
  private readonly transport: ReturnType<typeof createClaimsTransport>;
  constructor(
    transport: ReturnType<typeof createClaimsTransport>,
    availabilityRequest: typeof checkSiteAvailability,
  ) {
    this.transport = transport;
    this.availability = createSiteAvailability(availabilityRequest);
  }
  clear() {
    this.sequence += 1;
    this.operation?.abort();
    this.availability.clear();
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
    if (
      !this.context ||
      this.busy ||
      this.availability.checking ||
      !claimCanCreate(this.context, input.newAddress)
    )
      return;
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
            ? { evidence: input.evidence.trim(), evidence_url: input.evidenceURL.trim() }
            : {}),
        },
        operation.signal,
      );
      if (sequence === this.sequence && !operation.signal.aborted) this.created = created;
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
