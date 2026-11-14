import type { AuditAction } from '../site-submission/site-submission.types.ts';

export const siteVisibilities = ['VISIBLE', 'HIDDEN', 'REMOVED'] as const;
export type SiteVisibility = (typeof siteVisibilities)[number];

export interface ManagedSiteAudit {
  readonly id: string;
  readonly action: AuditAction;
}

export interface ManagedSite {
  readonly short_id: string;
  readonly custom_id?: string;
  readonly name: string;
  readonly url: string;
  readonly access_scope: 'CN_ONLY' | 'GLOBAL_ONLY' | 'ALL';
  readonly visibility: SiteVisibility;
  readonly visibility_reason?: string;
  readonly revision: number;
  readonly joined_at: string;
  readonly updated_at: string;
  readonly pending_audit?: ManagedSiteAudit;
}

export interface ManagedSitePage {
  readonly items: readonly ManagedSite[];
  readonly page: number;
  readonly page_size: number;
  readonly total_items: number;
  readonly total_pages: number;
}
