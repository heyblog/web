import type { SiteSearchResult } from '../../api/site-submission/site-submission.types.ts';

import { checkSiteAvailability, SiteSubmissionProblem } from './site-submission.api.browser.ts';

export interface SiteAddressContext {
  readonly url: string;
  readonly currentSiteShortID?: string;
}

export type SiteAvailabilityState =
  'idle' | 'checking' | 'available' | 'duplicate' | 'failed' | 'invalid';
export type SiteAvailabilityController = ReturnType<typeof createSiteAvailability>;

export function createSiteAvailability(request = checkSiteAvailability) {
  return new SiteAddressCheck(request);
}

class SiteAddressCheck {
  state = $state<SiteAvailabilityState>('idle');
  existingSite = $state.raw<SiteSearchResult | null>(null);
  message = $state('');
  checking = $derived(this.state === 'checking');
  private key = '';
  private sequence = 0;
  private operation: AbortController | null = null;
  private readonly request: typeof checkSiteAvailability;

  constructor(request: typeof checkSiteAvailability) {
    this.request = request;
  }

  setContext(context: SiteAddressContext): void {
    const key = `${context.currentSiteShortID ?? ''}\0${context.url.trim()}`;
    if (key === this.key) return;
    this.clear();
    this.key = key;
  }

  clear(): void {
    this.sequence += 1;
    this.operation?.abort();
    this.state = 'idle';
    this.existingSite = null;
    this.message = '';
  }

  async check(context: SiteAddressContext, force = false): Promise<boolean> {
    this.setContext(context);
    const url = context.url.trim();
    const parsed = URL.parse(url);
    if (
      !parsed ||
      (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') ||
      parsed.username ||
      parsed.password
    ) {
      this.state = 'invalid';
      this.message = '请填写有效的 HTTP 或 HTTPS 主页地址。';
      return false;
    }
    if (!force && this.state === 'available') return true;
    if (!force && this.state === 'duplicate') return false;
    this.operation?.abort();
    const operation = new AbortController();
    this.operation = operation;
    const sequence = ++this.sequence;
    this.state = 'checking';
    this.existingSite = null;
    this.message = '';
    try {
      const result = await this.request(url, {
        signal: operation.signal,
        currentSiteShortID: context.currentSiteShortID,
      });
      if (sequence !== this.sequence || operation.signal.aborted) return false;
      this.existingSite = result.existing_site ?? null;
      this.state = result.available ? 'available' : 'duplicate';
      return result.available;
    } catch (error) {
      if (sequence !== this.sequence || operation.signal.aborted) return false;
      if (!(error instanceof Error)) throw error;
      const invalid =
        error instanceof SiteSubmissionProblem && error.code === 'invalid_site_address';
      this.state = invalid ? 'invalid' : 'failed';
      this.message = invalid ? error.message : '暂时无法检查站点地址，请稍后重试。';
      return false;
    }
  }

  dispose(): void {
    this.clear();
  }
}
