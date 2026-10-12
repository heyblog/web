import { ClaimProblem, createClaimsTransport } from '../../api/site-management/claims.browser.ts';
import type { Claim, OwnershipDetail } from '../../api/site-management/claims.types.ts';

import { claimMessage } from './claims.model.ts';
export function createClaimReview(
  initial: Claim,
  ownership: OwnershipDetail | null,
  transport = createClaimsTransport(),
) {
  return new ClaimReview(initial, ownership, transport);
}
class ClaimReview {
  claim = $state<Claim>();
  ownership = $state<OwnershipDetail | null>(null);
  busy = $state(false);
  error = $state('');
  notice = $state('');
  private readonly operation = new AbortController();
  private readonly transport: ReturnType<typeof createClaimsTransport>;
  constructor(
    initial: Claim,
    ownership: OwnershipDetail | null,
    transport: ReturnType<typeof createClaimsTransport>,
  ) {
    this.claim = initial;
    this.ownership = ownership;
    this.transport = transport;
  }
  private async reload() {
    if (!this.claim) return;
    const [claim, ownership] = await Promise.all([
      this.transport.detail(this.claim.id, this.operation.signal, true),
      this.transport.ownership(this.claim.short_id, this.operation.signal),
    ]);
    this.claim = claim;
    this.ownership = ownership;
  }
  async refresh() {
    if (this.busy) return;
    this.busy = true;
    this.error = '';
    try {
      await this.reload();
    } catch (error) {
      this.report(error);
    } finally {
      this.busy = false;
    }
  }
  async review(approve: boolean, reason: string) {
    if (!this.claim || this.busy || !reason.trim()) return;
    this.busy = true;
    this.error = '';
    this.notice = '';
    try {
      this.claim = await this.transport.review(
        this.claim.id,
        { approve, reason: reason.trim() },
        this.operation.signal,
      );
      this.notice = '审核结果已保存。';
      await this.reload();
    } catch (error) {
      this.report(error);
      await this.recover();
    } finally {
      this.busy = false;
    }
  }
  async revoke(input: { readonly reason: string; readonly evidence: string }) {
    if (!this.ownership?.current || !this.ownership.can_revoke || !this.claim || this.busy) return;
    this.busy = true;
    this.error = '';
    this.notice = '';
    try {
      await this.transport.revoke(
        this.claim.short_id,
        {
          ownership_id: this.ownership.current.id,
          revision: this.ownership.current.revision,
          reason: input.reason.trim(),
          evidence: input.evidence.trim(),
        },
        this.operation.signal,
      );
      this.notice = '站点归属已撤销。';
      this.ownership = null;
      await this.reload();
    } catch (error) {
      this.report(error);
      await this.recover();
    } finally {
      this.busy = false;
    }
  }
  private async recover() {
    try {
      await this.reload();
    } catch (error) {
      if (!(error instanceof ClaimProblem)) throw error;
    }
  }
  private report(error: unknown) {
    if (error instanceof ClaimProblem) this.error = claimMessage(error.code);
    else throw error;
  }
  dispose() {
    this.operation.abort();
  }
}
