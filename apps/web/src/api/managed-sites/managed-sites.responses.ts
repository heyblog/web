import { type AuditAction, auditActions } from '../site-submission/site-submission.types.ts';

import {
  type ManagedSite,
  type ManagedSiteAudit,
  type ManagedSitePage,
  siteVisibilities,
  type SiteVisibility,
} from './managed-sites.types.ts';

function isRecord(value: unknown): value is Readonly<Record<string, unknown>> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isOptionalString(value: unknown): value is string | undefined {
  return value === undefined || typeof value === 'string';
}

function isTimestamp(value: unknown): value is string {
  return typeof value === 'string' && !Number.isNaN(Date.parse(value));
}

function isURL(value: unknown): value is string {
  if (typeof value !== 'string') return false;
  try {
    const url = new URL(value);
    return url.protocol === 'http:' || url.protocol === 'https:';
  } catch {
    return false;
  }
}

function isVisibility(value: unknown): value is SiteVisibility {
  return siteVisibilities.some((visibility) => visibility === value);
}

function isAuditAction(value: unknown): value is AuditAction {
  return auditActions.some((action) => action === value);
}

function parsePendingAudit(value: unknown): ManagedSiteAudit | null {
  if (!isRecord(value) || typeof value.id !== 'string' || !value.id || !isAuditAction(value.action))
    return null;
  return { id: value.id, action: value.action };
}

export function parseManagedSite(value: unknown): ManagedSite | null {
  if (
    !isRecord(value) ||
    typeof value.short_id !== 'string' ||
    !/^[0-9A-Za-z]{9}$/u.test(value.short_id) ||
    !isOptionalString(value.custom_id) ||
    typeof value.name !== 'string' ||
    !value.name ||
    !isURL(value.url) ||
    (value.access_scope !== 'CN_ONLY' &&
      value.access_scope !== 'GLOBAL_ONLY' &&
      value.access_scope !== 'ALL') ||
    !isVisibility(value.visibility) ||
    !isOptionalString(value.visibility_reason) ||
    !Number.isSafeInteger(value.revision) ||
    Number(value.revision) < 1 ||
    !isTimestamp(value.joined_at) ||
    !isTimestamp(value.updated_at)
  )
    return null;
  const pendingAudit = value.pending_audit;
  const parsedPendingAudit =
    pendingAudit === undefined ? undefined : parsePendingAudit(pendingAudit);
  if (pendingAudit !== undefined && parsedPendingAudit === null) return null;
  return {
    short_id: value.short_id,
    ...(value.custom_id ? { custom_id: value.custom_id } : {}),
    name: value.name,
    url: value.url,
    access_scope: value.access_scope,
    visibility: value.visibility,
    ...(value.visibility_reason ? { visibility_reason: value.visibility_reason } : {}),
    revision: Number(value.revision),
    joined_at: value.joined_at,
    updated_at: value.updated_at,
    ...(parsedPendingAudit ? { pending_audit: parsedPendingAudit } : {}),
  };
}

export function parseManagedSitePage(value: unknown): ManagedSitePage | null {
  if (
    !isRecord(value) ||
    !Array.isArray(value.items) ||
    !Number.isSafeInteger(value.page) ||
    Number(value.page) < 1 ||
    !Number.isSafeInteger(value.page_size) ||
    Number(value.page_size) < 1 ||
    !Number.isSafeInteger(value.total_items) ||
    Number(value.total_items) < 0 ||
    !Number.isSafeInteger(value.total_pages) ||
    Number(value.total_pages) < 1
  )
    return null;
  const items = value.items.map(parseManagedSite);
  if (items.some((item) => item === null)) return null;
  return {
    items: items.filter((item) => item !== null),
    page: Number(value.page),
    page_size: Number(value.page_size),
    total_items: Number(value.total_items),
    total_pages: Number(value.total_pages),
  };
}
