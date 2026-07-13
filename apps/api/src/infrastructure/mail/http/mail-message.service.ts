import type { AppConfig } from '@/infrastructure/app/http/app-config.service';

import type { MailMessage } from './mail.types';

const CRLF = '\r\n';

const encodeHeader = (value: string): string =>
  `=?UTF-8?B?${Buffer.from(value, 'utf8').toString('base64')}?=`;

const encodeBody = (value: string): string =>
  Buffer.from(value, 'utf8')
    .toString('base64')
    .replace(/(.{1,76})/g, '$1\r\n')
    .trim();

export const buildMimeMessage = (config: AppConfig, message: MailMessage): string =>
  [
    `From: ${config.API_SMTP_FROM}`,
    `To: ${message.to}`,
    `Subject: ${encodeHeader(message.subject)}`,
    'MIME-Version: 1.0',
    'Content-Type: text/plain; charset=UTF-8',
    'Content-Transfer-Encoding: base64',
    '',
    encodeBody(message.text),
    '',
  ].join(CRLF);
