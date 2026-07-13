import { afterEach, describe, expect, it, vi } from 'vitest';

const { sendMailThroughSmtpMock } = vi.hoisted(() => ({
  sendMailThroughSmtpMock: vi.fn(),
}));

vi.mock('@/infrastructure/mail/http/mail-smtp.service', () => ({
  sendMailThroughSmtp: sendMailThroughSmtpMock,
}));

import { sendSubmissionDecisionMail } from '@/infrastructure/sites/http/submission-decision-mail.service';
import { TEST_CONFIG } from '@tests/config';

describe('submission decision mail service', () => {
  afterEach(() => {
    sendMailThroughSmtpMock.mockReset();
  });

  it('builds the submission query url from the configured public web base url', async () => {
    await sendSubmissionDecisionMail(
      {
        ...TEST_CONFIG,
        API_WEB_BASE_URL: 'http://127.0.0.1:9101',
        WEB_PUBLIC_BASE_URL: 'https://www.zhblogs.net',
      },
      {
        recipient: 'author@example.com',
        auditId: 'audit-123',
        siteName: 'Example Blog',
        action: 'UPDATE',
        status: 'REJECTED',
        reviewerComment: '请补充截图',
      },
    );

    expect(sendMailThroughSmtpMock).toHaveBeenCalledWith(
      expect.objectContaining({
        WEB_PUBLIC_BASE_URL: 'https://www.zhblogs.net',
      }),
      expect.objectContaining({
        to: 'author@example.com',
        text: expect.stringContaining(
          'https://www.zhblogs.net/site/submit/query?audit_id=audit-123',
        ),
      }),
    );
  });
});
