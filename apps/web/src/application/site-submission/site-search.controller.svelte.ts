import { parseSiteSearch } from '../../api/site-submission/site-search.types.ts';
import { requestSubmissionSearch } from '../../api/site-submission/site-submission.browser.ts';
import type { SiteSearchResult } from '../../api/site-submission/site-submission.types.ts';

export function createSiteSearch(request = requestSubmissionSearch) {
  return new SiteSearch(request);
}
class SiteSearch {
  results = $state<readonly SiteSearchResult[]>([]);
  searching = $state(false);
  searched = $state(false);
  error = $state('');
  private sequence = 0;
  private controller: AbortController | null = null;
  private timer: ReturnType<typeof setTimeout> | undefined;
  private readonly request: typeof requestSubmissionSearch;
  constructor(request: typeof requestSubmissionSearch) {
    this.request = request;
  }
  clear() {
    this.sequence += 1;
    this.controller?.abort();
    clearTimeout(this.timer);
    this.results = [];
    this.searching = false;
    this.searched = false;
    this.error = '';
  }
  schedule(query: string) {
    this.clear();
    if (query.trim())
      this.timer = setTimeout(() => {
        void this.search(query);
      }, 250);
  }
  async search(query: string) {
    this.clear();
    const value = query.trim();
    if (!value) return;
    const sequence = this.sequence;
    const controller = new AbortController();
    this.controller = controller;
    this.searching = true;
    try {
      const response = await this.request(value, controller.signal);
      if (!response.ok) {
        if (sequence === this.sequence) this.error = '站点搜索暂时不可用，请重试。';
        return;
      }
      const items = parseSiteSearch(await response.json());
      if (sequence === this.sequence) {
        this.results = items;
        this.searched = true;
      }
    } catch (error) {
      if (error instanceof Error) {
        if (sequence === this.sequence && !controller.signal.aborted)
          this.error = '站点搜索暂时不可用，请重试。';
      } else throw error;
    } finally {
      if (sequence === this.sequence) this.searching = false;
    }
  }
  dispose() {
    this.clear();
  }
}
