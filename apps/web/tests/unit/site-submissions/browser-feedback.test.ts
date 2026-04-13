import { beforeEach, describe, expect, it, vi } from 'vitest';

import {
  buildSubmissionQueryHref,
  clearSubmissionIdentifierSearchParams,
  copySubmissionAuditId,
} from '@/application/site-submission/site-submission.browser-feedback';
import { getWebBaseUrl } from '@tests/setup/env';

describe('site submission browser feedback helpers', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('copies the audit id when possible', async () => {
    const writeText = vi.fn(async () => undefined);

    vi.stubGlobal('navigator', {
      clipboard: {
        writeText,
      },
    });

    await copySubmissionAuditId('audit-001');

    expect(writeText).toHaveBeenCalledWith('audit-001');
  });

  it('ignores clipboard write failures', async () => {
    const writeText = vi.fn(async () => {
      throw new Error('clipboard failed');
    });

    vi.stubGlobal('navigator', {
      clipboard: {
        writeText,
      },
    });
    await expect(copySubmissionAuditId(' audit-002 ')).resolves.toBeUndefined();

    expect(writeText).toHaveBeenCalledWith('audit-002');
  });

  it('removes the canonical site_id query param without touching other search params', () => {
    const replaceState = vi.fn();

    vi.stubGlobal('window', {
      location: {
        href: `${getWebBaseUrl()}/site/submit/delete?site_id=site-1&foo=bar#done`,
      },
      history: {
        state: { from: 'test' },
        replaceState,
      },
    });

    clearSubmissionIdentifierSearchParams();

    expect(replaceState).toHaveBeenCalledWith(
      { from: 'test' },
      '',
      '/site/submit/delete?foo=bar#done',
    );
  });

  it('builds query-page hrefs from audit ids', () => {
    expect(buildSubmissionQueryHref(' audit-003 ')).toBe('/site/submit/query?audit_id=audit-003');
  });
});
