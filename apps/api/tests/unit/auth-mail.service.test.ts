import { afterEach, describe, expect, it, vi } from 'vitest';

const { sendMailThroughSmtpMock } = vi.hoisted(() => ({
  sendMailThroughSmtpMock: vi.fn(),
}));

vi.mock('@/infrastructure/mail/http/mail-smtp.service', () => ({
  sendMailThroughSmtp: sendMailThroughSmtpMock,
}));

import {
  sendPasswordResetMail,
  sendVerificationMail,
} from '@/infrastructure/auth/http/auth-mail.service';
import { TEST_CONFIG } from '@tests/config';

describe('auth mail service', () => {
  afterEach(() => {
    sendMailThroughSmtpMock.mockReset();
  });

  it('uses the configured public web base url in verification mails', async () => {
    await sendVerificationMail(
      {
        ...TEST_CONFIG,
        API_WEB_BASE_URL: 'http://127.0.0.1:9101',
        WEB_PUBLIC_BASE_URL: 'https://www.zhblogs.net/',
      },
      {
        recipient: 'user@example.com',
        nickname: 'User',
        token: 'verify-token',
        nextPath: '/management/site-submissions?tab=pending',
      },
    );

    expect(sendMailThroughSmtpMock).toHaveBeenCalledWith(
      expect.objectContaining({
        WEB_PUBLIC_BASE_URL: 'https://www.zhblogs.net/',
      }),
      expect.objectContaining({
        to: 'user@example.com',
        text: expect.stringContaining(
          'https://www.zhblogs.net/verify-email?token=verify-token&next=%2Fmanagement%2Fsite-submissions%3Ftab%3Dpending',
        ),
      }),
    );
  });

  it('omits the next query when password reset mail has no return path', async () => {
    await sendPasswordResetMail(
      {
        ...TEST_CONFIG,
        API_WEB_BASE_URL: 'http://127.0.0.1:9101',
        WEB_PUBLIC_BASE_URL: 'https://www.zhblogs.net',
      },
      {
        recipient: 'user@example.com',
        nickname: 'User',
        token: 'reset-token',
      },
    );

    expect(sendMailThroughSmtpMock).toHaveBeenCalledWith(
      expect.objectContaining({
        WEB_PUBLIC_BASE_URL: 'https://www.zhblogs.net',
      }),
      expect.objectContaining({
        to: 'user@example.com',
        text: expect.stringContaining('https://www.zhblogs.net/reset-password?token=reset-token'),
      }),
    );
    expect(sendMailThroughSmtpMock.mock.calls.at(-1)?.[1].text).not.toContain('&next=');
  });
});
