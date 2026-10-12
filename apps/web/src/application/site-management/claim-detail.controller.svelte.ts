import { ClaimProblem, createClaimsTransport } from '../../api/site-management/claims.browser.ts';
import type { Claim, ClaimChallenge } from '../../api/site-management/claims.types.ts';

import { claimMessage } from './claims.model.ts';

export function createClaimDetail(
  initial: Claim,
  challenge?: ClaimChallenge,
  transport = createClaimsTransport(),
) {
  return new ClaimDetail(initial, challenge, transport);
}
class ClaimDetail {
  claim = $state<Claim>();
  challenge = $state<ClaimChallenge | undefined>();
  busy = $state<'refresh' | 'check' | 'cancel' | 'regenerate' | null>(null);
  error = $state('');
  notice = $state('');
  private readonly operation = new AbortController();
  private readonly transport: ReturnType<typeof createClaimsTransport>;
  constructor(
    initial: Claim,
    challenge: ClaimChallenge | undefined,
    transport: ReturnType<typeof createClaimsTransport>,
  ) {
    this.claim = initial;
    this.challenge = challenge;
    this.transport = transport;
  }
  private accept(claim: Claim) {
    if (claim.status !== 'PENDING' || claim.expires_at !== this.challenge?.claim.expires_at)
      this.challenge = undefined;
    this.claim = claim;
  }
  async run(action: 'refresh' | 'check' | 'cancel' | 'regenerate') {
    if (!this.claim || this.busy) return;
    this.busy = action;
    this.error = '';
    this.notice = '';
    try {
      if (action === 'regenerate') {
        const result = await this.transport.regenerate(this.claim.id, this.operation.signal);
        this.accept(result.claim);
        this.challenge = result;
        this.notice = '验证配置已更新，请替换之前的配置。';
      } else if (action === 'refresh')
        this.accept(await this.transport.detail(this.claim.id, this.operation.signal));
      else {
        this.accept(await this.transport.change(this.claim.id, action, this.operation.signal));
        this.notice = action === 'cancel' ? '申请已取消。' : '验证通过。';
      }
    } catch (error) {
      if (error instanceof ClaimProblem) {
        this.error = claimMessage(error.code);
        if (error.code === 'proof_changed') this.challenge = undefined;
        if (action !== 'refresh' && !this.operation.signal.aborted) {
          try {
            this.accept(await this.transport.detail(this.claim.id, this.operation.signal));
          } catch (refreshError) {
            if (!(refreshError instanceof ClaimProblem)) throw refreshError;
          }
        }
      } else throw error;
    } finally {
      this.busy = null;
    }
  }
  dispose() {
    this.operation.abort();
    this.challenge = undefined;
  }
}
