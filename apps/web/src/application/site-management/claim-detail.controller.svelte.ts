import { ClaimProblem, createClaimsTransport } from '../../api/site-management/claims.browser.ts';
import type {
  Claim,
  ClaimChallenge,
  VerificationTask,
} from '../../api/site-management/claims.types.ts';

import { claimMessage } from './claims.model.ts';

type Value<T> = T | (() => T);

function readValue<T>(value: Value<T>): T {
  return typeof value === 'function' ? (value as () => T)() : value;
}

export function createClaimDetail(
  initial: Value<Claim>,
  challenge?: Value<ClaimChallenge | undefined>,
  transport = createClaimsTransport(),
) {
  return new ClaimDetail(
    readValue(initial),
    challenge === undefined ? undefined : readValue(challenge),
    transport,
  );
}
class ClaimDetail {
  claim = $state<Claim>();
  challenge = $state<ClaimChallenge | undefined>();
  busy = $state<'refresh' | 'check' | 'cancel' | 'regenerate' | null>(null);
  error = $state('');
  notice = $state('');
  task = $state<VerificationTask | null>(null);
  polling = $state(false);
  private pendingPoll: Promise<void> | null = null;
  private timer: ReturnType<typeof setInterval> | undefined;
  private readonly operation = new AbortController();
  private readonly transport: ReturnType<typeof createClaimsTransport>;
  constructor(
    initial: Claim,
    challenge: ClaimChallenge | undefined,
    transport: ReturnType<typeof createClaimsTransport>,
  ) {
    this.claim = initial;
    this.task = initial.task ?? null;
    this.challenge = challenge;
    this.transport = transport;
  }
  private accept(claim: Claim) {
    if (this.operation.signal.aborted) return;
    if (this.claim?.expires_at !== claim.expires_at) this.task = null;
    if (claim.status !== 'PENDING' || claim.expires_at !== this.challenge?.claim.expires_at)
      this.challenge = undefined;
    this.claim = claim;
    if (claim.task) this.task = claim.task;
    if (claim.status !== 'PENDING') this.task = null;
  }
  get checking() {
    return (
      this.task !== null &&
      ['queued', 'running', 'pausing', 'paused', 'cancelling'].includes(this.task.state)
    );
  }
  poll(): Promise<void> {
    if (
      !this.claim ||
      this.busy ||
      this.polling ||
      document.hidden ||
      this.operation.signal.aborted
    )
      return Promise.resolve();
    this.polling = true;
    this.pendingPoll = this.loadTask().finally(() => {
      this.polling = false;
      this.pendingPoll = null;
    });
    return this.pendingPoll;
  }
  private async loadTask() {
    if (!this.claim) return;
    try {
      const task = await this.transport.task(this.claim.id, this.operation.signal);
      if (this.operation.signal.aborted) return;
      this.task = task;
      if (!this.checking) {
        this.accept(await this.transport.detail(this.claim.id, this.operation.signal));
        if (this.operation.signal.aborted) return;
        if (this.claim.status === 'VERIFIED') this.notice = '验证通过。';
        else if (this.task)
          this.notice =
            this.task.state === 'succeeded'
              ? '尚未找到匹配的验证证明。修正后可再次检查。'
              : '本次检查未完成，请刷新确认后重试。';
      }
    } catch (error) {
      if (this.operation.signal.aborted) return;
      if (error instanceof ClaimProblem) this.error = claimMessage(error.code);
      else throw error;
    }
  }
  startPolling() {
    if (this.timer !== undefined) return;
    if (this.claim?.method !== 'FILE' && this.claim?.method !== 'META') return;
    void this.poll();
    this.timer = setInterval(() => {
      if (this.checking) void this.poll();
    }, 3000);
    document.addEventListener('visibilitychange', this.onVisible);
  }
  private readonly onVisible = () => {
    if (this.checking) void this.poll();
  };
  async run(action: 'refresh' | 'check' | 'cancel' | 'regenerate') {
    if (!this.claim || this.busy || this.operation.signal.aborted) return;
    this.busy = action;
    try {
      await this.pendingPoll;
      if (this.operation.signal.aborted) return;
      this.error = '';
      this.notice = '';
      if (action === 'regenerate') {
        const result = await this.transport.regenerate(this.claim.id, this.operation.signal);
        if (this.operation.signal.aborted) return;
        this.accept(result.claim);
        this.challenge = result;
        this.task = null;
        this.notice = '验证配置已更新，请替换之前的配置。';
      } else if (action === 'refresh') {
        this.accept(await this.transport.detail(this.claim.id, this.operation.signal));
        if (this.claim.method === 'FILE' || this.claim.method === 'META')
          this.task = await this.transport.task(this.claim.id, this.operation.signal);
      } else {
        this.accept(await this.transport.change(this.claim.id, action, this.operation.signal));
        this.notice =
          action === 'cancel'
            ? '申请已取消。'
            : this.claim.status === 'VERIFIED'
              ? '验证通过。'
              : '验证检查已提交。';
      }
    } catch (error) {
      if (this.operation.signal.aborted) return;
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
    clearInterval(this.timer);
    if (this.timer !== undefined) {
      document.removeEventListener('visibilitychange', this.onVisible);
      this.timer = undefined;
    }
    this.operation.abort();
    this.challenge = undefined;
  }
}
