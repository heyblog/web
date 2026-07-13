import { SiteAudits } from '@zhblogs/db';

import { afterEach, describe, expect, it, vi } from 'vitest';

const { sendSubmissionDecisionMailMock } = vi.hoisted(() => ({
  sendSubmissionDecisionMailMock: vi.fn(),
}));

vi.mock('@/infrastructure/sites/http/submission-decision-mail.service', async () => {
  const actual = await vi.importActual<
    typeof import('@/infrastructure/sites/http/submission-decision-mail.service')
  >('@/infrastructure/sites/http/submission-decision-mail.service');

  return {
    ...actual,
    canSendSubmissionDecisionMail: () => true,
    sendSubmissionDecisionMail: sendSubmissionDecisionMailMock,
  };
});

import { createTestApp } from '@tests/create-test-app';
import { mockReadSelect } from '@tests/fixtures/db-mocks';

import {
  buildManagedSiteSnapshot,
  MANAGEMENT_TEST_IDS,
  mockManagementUser,
} from './site-test.helpers';

describe('management site audit review mail routes', () => {
  let app: ReturnType<typeof createTestApp> | undefined;

  afterEach(async () => {
    sendSubmissionDecisionMailMock.mockReset();
    vi.restoreAllMocks();
    await app?.close();
    app = undefined;
  });

  it('uses the configured public web base url in review mails', async () => {
    app = createTestApp({
      disableExternalServices: true,
      envOverrides: {
        API_WEB_BASE_URL: 'http://127.0.0.1:9101',
        WEB_PUBLIC_BASE_URL: 'https://www.zhblogs.net',
      },
    });

    await app.ready();
    mockManagementUser(app, 'site_audit.review');

    mockReadSelect(app, [
      {
        table: SiteAudits,
        rows: [
          {
            id: MANAGEMENT_TEST_IDS.auditId,
            action: 'UPDATE',
            status: 'PENDING',
            site_id: MANAGEMENT_TEST_IDS.siteId,
            submit_reason: '补充信息后再审',
            submitter_email: 'author@example.com',
            notify_by_email: true,
            current_snapshot: buildManagedSiteSnapshot(),
            proposed_snapshot: buildManagedSiteSnapshot({
              name: 'Example Blog',
            }),
          },
        ],
      },
    ]);

    app.db.write.update = vi.fn((_table: unknown) => ({
      set: vi.fn(() => ({
        where: vi.fn(() => ({
          returning: vi.fn(async () => [
            {
              id: MANAGEMENT_TEST_IDS.auditId,
              action: 'UPDATE',
              status: 'REJECTED',
              site_id: MANAGEMENT_TEST_IDS.siteId,
              submitter_email: 'author@example.com',
              notify_by_email: true,
              reviewer_comment: '请补充审核原因截图',
              current_snapshot: buildManagedSiteSnapshot(),
              proposed_snapshot: buildManagedSiteSnapshot({
                name: 'Example Blog',
              }),
            },
          ]),
        })),
      })),
    })) as unknown as typeof app.db.write.update;

    const response = await app.inject({
      method: 'POST',
      url: `/api/management/site-audits/${MANAGEMENT_TEST_IDS.auditId}/review`,
      payload: {
        decision: 'REJECTED',
        reviewer_comment: '请补充审核原因截图',
      },
    });

    expect(response.statusCode).toBe(200);
    expect(sendSubmissionDecisionMailMock).toHaveBeenCalledWith(
      expect.objectContaining({
        WEB_PUBLIC_BASE_URL: 'https://www.zhblogs.net',
      }),
      {
        recipient: 'author@example.com',
        auditId: MANAGEMENT_TEST_IDS.auditId,
        siteName: 'Example Blog',
        action: 'UPDATE',
        status: 'REJECTED',
        reviewerComment: '请补充审核原因截图',
      },
    );
  });
});
