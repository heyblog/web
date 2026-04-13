import { afterEach, describe, expect, it, vi } from 'vitest';

import { POST as reviewPost } from '@/pages/management/site-submissions/[auditId]/review';
import { POST as siteUpdatePost } from '@/pages/management/sites/[siteId]/update';
import { getWebBaseUrl } from '@tests/setup/env';

const siteId = '11111111-1111-4111-8111-111111111111';
const auditId = '22222222-2222-4222-8222-222222222222';

const createSiteUpdateContext = (request: Request, currentSiteId = siteId) =>
  ({
    request,
    params: {
      siteId: currentSiteId,
    },
  }) as unknown as Parameters<typeof siteUpdatePost>[0];

const createReviewContext = (request: Request, currentAuditId = auditId) =>
  ({
    request,
    params: {
      auditId: currentAuditId,
    },
    redirect: (location: string, status = 302) =>
      new Response(null, {
        status,
        headers: {
          location,
        },
      }),
  }) as unknown as Parameters<typeof reviewPost>[0];

describe('management site routes', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('returns JSON success payload for site updates', async () => {
    const fetchMock = vi.fn(
      async () =>
        new Response(
          JSON.stringify({
            ok: true,
            data: {
              site_id: siteId,
            },
          }),
        ),
    );
    vi.stubGlobal('fetch', fetchMock);

    const response = await siteUpdatePost(
      createSiteUpdateContext(
        new Request(`${getWebBaseUrl()}/management/sites/${siteId}/update`, {
          method: 'POST',
          headers: {
            accept: 'application/json',
            'content-type': 'application/json',
          },
          body: JSON.stringify({
            snapshot: {
              name: 'Example Blog',
              url: 'https://example.com',
              is_show: true,
            },
            comment: 'admin update',
          }),
        }),
      ),
    );

    expect(fetchMock).toHaveBeenCalledWith(
      `${getWebBaseUrl()}/api/management/sites/${siteId}`,
      expect.objectContaining({
        method: 'PUT',
        body: JSON.stringify({
          snapshot: {
            name: 'Example Blog',
            url: 'https://example.com',
            is_show: true,
          },
          comment: 'admin update',
        }),
      }),
    );
    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({
      ok: true,
      status: 'site_updated',
      target: siteId,
      redirect: null,
      message: '',
      data: {
        site_id: siteId,
      },
    });
  });

  it('returns JSON unauthorized payload for site updates', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(
        async () =>
          new Response(
            JSON.stringify({
              error: {
                message: 'expired',
              },
            }),
            { status: 401 },
          ),
      ),
    );

    const response = await siteUpdatePost(
      createSiteUpdateContext(
        new Request(`${getWebBaseUrl()}/management/sites/${siteId}/update`, {
          method: 'POST',
          headers: {
            accept: 'application/json',
            'content-type': 'application/json',
          },
          body: JSON.stringify({
            snapshot: {
              name: 'Example Blog',
              url: 'https://example.com',
            },
          }),
        }),
      ),
    );

    expect(response.status).toBe(401);
    expect(await response.json()).toEqual({
      ok: false,
      code: 'unauthorized',
      target: siteId,
      redirect: `/login?next=${encodeURIComponent(`/management/sites/${siteId}`)}`,
      message: '登录状态已过期，请重新登录。',
      data: null,
    });
  });

  it('returns JSON error payload for forbidden site updates', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(
        async () =>
          new Response(
            JSON.stringify({
              error: {
                message: '  Read-only site fields cannot be modified manually: reason.  ',
              },
            }),
            { status: 403 },
          ),
      ),
    );

    const response = await siteUpdatePost(
      createSiteUpdateContext(
        new Request(`${getWebBaseUrl()}/management/sites/${siteId}/update`, {
          method: 'POST',
          headers: {
            accept: 'application/json',
            'content-type': 'application/json',
          },
          body: JSON.stringify({
            snapshot: {
              name: 'Example Blog',
              url: 'https://example.com',
              reason: 'manual override',
            },
          }),
        }),
      ),
    );

    expect(response.status).toBe(403);
    expect(await response.json()).toEqual({
      ok: false,
      code: 'site_update_failed',
      target: siteId,
      redirect: null,
      message: 'Read-only site fields cannot be modified manually: reason.',
      data: null,
    });
  });

  it('returns JSON success payload for audit reviews with snapshot override', async () => {
    const fetchMock = vi.fn(
      async () =>
        new Response(
          JSON.stringify({
            ok: true,
            data: {
              audit_id: auditId,
              action: 'UPDATE',
              status: 'APPROVED',
              site_id: siteId,
            },
          }),
        ),
    );
    vi.stubGlobal('fetch', fetchMock);

    const response = await reviewPost(
      createReviewContext(
        new Request(`${getWebBaseUrl()}/management/site-submissions/${auditId}/review`, {
          method: 'POST',
          headers: {
            accept: 'application/json',
            'content-type': 'application/json',
          },
          body: JSON.stringify({
            decision: 'APPROVED',
            reviewer_comment: 'apply reviewer override',
            snapshot_override: {
              name: 'Example Blog',
              url: 'https://example.com',
              recommend: true,
            },
          }),
        }),
      ),
    );

    expect(fetchMock).toHaveBeenCalledWith(
      `${getWebBaseUrl()}/api/management/site-audits/${auditId}/review`,
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({
          decision: 'APPROVED',
          reviewer_comment: 'apply reviewer override',
          snapshot_override: {
            name: 'Example Blog',
            url: 'https://example.com',
            recommend: true,
          },
        }),
      }),
    );
    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({
      ok: true,
      status: 'reviewed',
      target: auditId,
      redirect: null,
      message: '',
      data: {
        audit_id: auditId,
        action: 'UPDATE',
        status: 'APPROVED',
        site_id: siteId,
      },
    });
  });

  it('returns JSON error payload for forbidden audit overrides', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(
        async () =>
          new Response(
            JSON.stringify({
              error: {
                message: '  Read-only site fields cannot be modified manually: from.  ',
              },
            }),
            { status: 403 },
          ),
      ),
    );

    const response = await reviewPost(
      createReviewContext(
        new Request(`${getWebBaseUrl()}/management/site-submissions/${auditId}/review`, {
          method: 'POST',
          headers: {
            accept: 'application/json',
            'content-type': 'application/json',
          },
          body: JSON.stringify({
            decision: 'APPROVED',
            snapshot_override: {
              name: 'Example Blog',
              url: 'https://example.com',
              from: ['TRAVELLINGS'],
            },
          }),
        }),
      ),
    );

    expect(response.status).toBe(403);
    expect(await response.json()).toEqual({
      ok: false,
      code: 'audit_review_failed',
      target: auditId,
      redirect: null,
      message: 'Read-only site fields cannot be modified manually: from.',
      data: null,
    });
  });

  it('returns JSON invalid payload when rejecting without reviewer comment', async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);

    const response = await reviewPost(
      createReviewContext(
        new Request(`${getWebBaseUrl()}/management/site-submissions/${auditId}/review`, {
          method: 'POST',
          headers: {
            accept: 'application/json',
            'content-type': 'application/json',
          },
          body: JSON.stringify({
            decision: 'REJECTED',
          }),
        }),
      ),
    );

    expect(fetchMock).not.toHaveBeenCalled();
    expect(response.status).toBe(400);
    expect(await response.json()).toEqual({
      ok: false,
      code: 'invalid_audit_review_request',
      target: auditId,
      redirect: null,
      message: '审核请求无效，请刷新后重试。',
      data: null,
    });
  });
});
