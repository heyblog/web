import type { AppConfig } from '@/infrastructure/app/http/app-config.service';
import { sendMailThroughSmtp } from '@/infrastructure/mail/http/mail-smtp.service';
import { buildPublicWebUrl } from '@/infrastructure/mail/http/public-web-url.service';

export interface SubmissionDecisionMailInput {
  recipient: string;
  auditId: string;
  siteName: string;
  action: string;
  status: 'APPROVED' | 'REJECTED';
  reviewerComment: string | null;
}

export const canSendSubmissionDecisionMail = (config: AppConfig): boolean =>
  Boolean(config.API_SMTP_HOST && config.API_SMTP_PORT && config.API_SMTP_FROM);

export async function sendSubmissionDecisionMail(
  config: AppConfig,
  payload: SubmissionDecisionMailInput,
): Promise<void> {
  if (!canSendSubmissionDecisionMail(config)) {
    return;
  }

  const subject =
    payload.status === 'APPROVED'
      ? `[集博栈] 站点审核提交已通过：${payload.siteName}`
      : `[集博栈] 站点审核提交未通过：${payload.siteName}`;
  const queryUrl = buildPublicWebUrl(config.WEB_PUBLIC_BASE_URL, '/site/submit/query', {
    audit_id: payload.auditId,
  });

  const text = [
    `站点：${payload.siteName}`,
    `动作：${payload.action}`,
    `结果：${payload.status === 'APPROVED' ? '已通过' : '已拒绝'}`,
    `审核编号：${payload.auditId}`,
    `查询地址：${queryUrl}`,
    payload.reviewerComment ? `审核备注：${payload.reviewerComment}` : null,
  ]
    .filter(Boolean)
    .join('\n');

  await sendMailThroughSmtp(config, {
    to: payload.recipient,
    subject,
    text,
  });
}
