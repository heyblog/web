import type { Claim, ClaimContext } from '../../api/site-management/claims.types.ts';

export function claimExpired(claim: Claim, now = Date.now()): boolean {
  return (
    claim.status === 'PENDING' && Boolean(claim.expires_at && Date.parse(claim.expires_at) <= now)
  );
}
export function claimLabel(claim: Claim, now = Date.now()): string {
  if (claimExpired(claim, now)) return '已过期';
  if (claim.status === 'PENDING') return claim.method === 'MANUAL' ? '待审核' : '待验证';
  return { VERIFIED: '验证完成', REJECTED: '未通过', CANCELLED: '已取消' }[claim.status];
}
export function claimDestination(claim: Claim): string | null {
  if (!claim.can_manage) return null;
  const path = `/dashboard/sites/${encodeURIComponent(claim.short_id)}`;
  return claim.status === 'VERIFIED' &&
    !claim.consumed_at &&
    claim.address !== claim.current_address
    ? `${path}?verified_claim=${encodeURIComponent(claim.id)}#information`
    : path;
}
export function claimCanCreate(context: ClaimContext, newAddress: boolean): boolean {
  return newAddress ? context.can_verify_address : context.can_claim;
}
const messages: Readonly<Record<string, string>> = {
  proof_not_found: '尚未找到匹配的验证配置。请确认已发布，再检查一次。',
  verification_network: '暂时无法访问验证地址，请确认站点可公开访问后重试。',
  verification_timeout: '验证超时，请稍后再检查。',
  proof_changed: '验证配置已更新，请重新读取申请并使用最新配置。',
  claim_expired: '验证配置已过期，请重新生成配置。',
  claim_finished: '申请已处理，请刷新查看最新结果。',
  ownership_changed: '站点归属已变化，请刷新后重新确认。',
  site_already_owned: '此站点已有有效归属，请刷新查看当前状态。',
  address_changed: '站点主页地址已变化，请重新选择站点发起验证。',
  address_owner_required: '请先认领当前主页地址，再验证新地址。',
  self_review: '自己的申请需要由其他审核者处理。',
  self_revoke: '自己的站点归属需要由其他审核者处理。',
  forbidden: '当前账号无权执行此操作。',
  unauthorized: '登录已过期，请重新登录。',
  invalid_address: '请填写有效的 HTTP 或 HTTPS 地址。',
  invalid_evidence: '请填写完整的归属说明或处理证据。',
  manual_evidence_too_short: '归属说明至少需要 20 个字符。',
  manual_evidence_required: '请提供公开证据链接或至少一个证明附件。',
  manual_attachment_required: '请提供公开证据链接或至少一个证明附件。',
  manual_attachment_count: '最多上传 3 个证明附件。',
  manual_attachment_type: '仅支持 PNG、JPEG 和 PDF 文件。',
  manual_attachment_too_large: '图片不能超过 5 MiB，PDF 不能超过 10 MiB。',
  manual_attachment_total_too_large: '证明附件总大小不能超过 20 MiB。',
  manual_attachment_invalid: '证明附件信息无效，请重新选择文件。',
  manual_attachment_limit: '证明附件上传次数已达上限，请稍后再试。',
  manual_submission_limit: '24 小时内最多提交 3 次人工认证申请。',
  manual_pending_limit: '当前已有 3 个待审核申请，请等待处理结果。',
  manual_rejected_cooldown: '申请被拒后 24 小时内不能再次提交同一站点。',
  attachment_upload_failed: '证明附件上传失败，请重试。',
  attachment_not_ready: '证明附件仍在处理，请稍后重试。',
  attachment_invalid: '证明附件内容无法验证，请重新选择文件。',
  attachment_bound: '证明附件已经提交到申请，不能再移除。',
  attachment_storage_unavailable: '证明附件服务暂时不可用，请稍后重试。',
  attachment_forbidden: '无权操作此证明附件。',
  invalid_evidence_url: '请填写有效的公开证据链接。',
  invalid_reason: '请填写处理理由。',
  claim_not_found: '申请或站点已不可用，请返回列表。',
  site_removed: '此站点已移除，无法发起认领。',
  rate_limited: '操作过于频繁，请稍后重试。',
  network_error: '连接中断，请刷新状态后再操作。',
  invalid_response: '结果暂时无法读取，请刷新状态后再操作。',
  cancelled: '操作已停止。',
};
export function claimMessage(code: string): string {
  return messages[code] ?? '操作未完成，请刷新状态后重试。';
}

export function verifiedClaimAddress(claim: Claim, shortId: string): string | undefined {
  if (
    claim.short_id !== shortId ||
    claim.status !== 'VERIFIED' ||
    !claim.can_manage ||
    claim.consumed_at ||
    claim.address === claim.current_address
  )
    return undefined;
  const url = URL.parse(claim.address);
  return url &&
    (url.protocol === 'http:' || url.protocol === 'https:') &&
    !url.username &&
    !url.password
    ? claim.address
    : undefined;
}
