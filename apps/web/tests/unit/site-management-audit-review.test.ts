import { afterEach, describe, expect, it, vi } from 'vitest';

import { POST as reviewPost } from '@/pages/management/site-submissions/[auditId]/review';
import { getApiBaseUrl, getWebBaseUrl } from '@tests/setup/env';

import { auditId, createReviewContext, siteId } from './site-management-routes.helpers';

describe('management audit review route', () => {
  afterEach(() => {
    vi.restoreAllMocks();
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
      `${getApiBaseUrl()}/api/management/site-audits/${auditId}/review`,
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

  it('forwards reviewer comment when rejecting with a filled reason', async () => {
    const fetchMock = vi.fn(
      async () =>
        new Response(
          JSON.stringify({
            ok: true,
            data: {
              audit_id: auditId,
              action: 'UPDATE',
              status: 'REJECTED',
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
            decision: 'REJECTED',
            reviewer_comment: '缺少审核所需说明',
          }),
        }),
      ),
    );

    expect(fetchMock).toHaveBeenCalledWith(
      `${getApiBaseUrl()}/api/management/site-audits/${auditId}/review`,
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({
          decision: 'REJECTED',
          reviewer_comment: '缺少审核所需说明',
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
        status: 'REJECTED',
        site_id: siteId,
      },
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
