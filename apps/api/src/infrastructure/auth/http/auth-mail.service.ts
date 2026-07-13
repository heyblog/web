import type { AppConfig } from '@/infrastructure/app/http/app-config.service';
import { sendMailThroughSmtp } from '@/infrastructure/mail/http/mail-smtp.service';
import { buildPublicWebUrl } from '@/infrastructure/mail/http/public-web-url.service';

type AuthMailBaseInput = {
  recipient: string;
  nickname: string;
  nextPath?: string | null;
};

export const sendVerificationMail = async (
  config: AppConfig,
  input: AuthMailBaseInput & { token: string },
): Promise<void> => {
  const verifyUrl = buildPublicWebUrl(config.WEB_PUBLIC_BASE_URL, '/verify-email', {
    token: input.token,
    ...(input.nextPath ? { next: input.nextPath } : {}),
  });

  await sendMailThroughSmtp(config, {
    to: input.recipient,
    subject: '[集博栈 - zhblogs] 验证你的邮箱',
    text: [
      `${input.nickname}，你好：`,
      '',
      '请点击下方的链接对邮箱进行验证，验证后方可登录：',
      verifyUrl,
      '',
      '如果该操作并非你本人发起，请忽略这封邮件。',
    ].join('\n'),
  });
};

export const sendPasswordResetMail = async (
  config: AppConfig,
  input: AuthMailBaseInput & { token: string },
): Promise<void> => {
  const resetUrl = buildPublicWebUrl(config.WEB_PUBLIC_BASE_URL, '/reset-password', {
    token: input.token,
    ...(input.nextPath ? { next: input.nextPath } : {}),
  });

  await sendMailThroughSmtp(config, {
    to: input.recipient,
    subject: '[集博栈 - zhblogs] 重置你的密码',
    text: [
      `${input.nickname}，你好：`,
      '',
      '请点击下方的链接重置密码：',
      resetUrl,
      '',
      '如果该操作并非你本人发起，请忽略这封邮件。',
    ].join('\n'),
  });
};
