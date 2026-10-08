<script lang="ts">
  import { accountRequest } from '@/api/site-management/site-management.browser';
  import { type Claim, parseClaim } from '@/api/site-management/site-management.types';
  import {
    claimStatusLabels,
    managementError,
    SiteManagementProblem,
  } from '@/application/site-management/messages';
  import InlineAlert from '@/components/feedback/InlineAlert.svelte';
  interface Props {
    initial: Claim;
  }
  let { initial }: Props = $props();
  let claim = $state.raw(initial);
  let reason = $state('');
  let evidence = $state('');
  let userId = $state('');
  let busy = $state(false);
  let error = $state('');
  let notice = $state('');
  async function operate(action: 'approve' | 'reject' | 'revoke' | 'reassign'): Promise<void> {
    if (busy || !reason.trim()) return;
    if (
      (action === 'revoke' || action === 'reassign') &&
      !confirm(action === 'revoke' ? '撤销此站点的有效归属？' : '将此站点归属转移给填写的用户？')
    )
      return;
    busy = true;
    error = '';
    notice = '';
    try {
      const review = action === 'approve' || action === 'reject';
      const response = await accountRequest(
        review ? `site-claims/${claim.id}/review` : `site-ownership/${claim.short_id}`,
        {
          management: true,
          method: review ? 'POST' : action === 'revoke' ? 'DELETE' : 'PUT',
          body: review
            ? { approve: action === 'approve', reason: reason.trim() }
            : {
                reason: reason.trim(),
                evidence: evidence.trim(),
                ...(action === 'reassign' ? { user_id: userId.trim() } : {}),
              },
        },
      );
      if (!response.ok) throw new SiteManagementProblem(await managementError(response));
      if (review) claim = parseClaim(await response.json());
      notice = review
        ? '审核结果已保存。'
        : action === 'revoke'
          ? '站点归属已撤销。'
          : '站点归属已转移。';
    } catch (caught) {
      error = caught instanceof SiteManagementProblem ? caught.message : '操作未完成，请稍后重试。';
    } finally {
      busy = false;
    }
  }
</script>

<section class="grid gap-5">
  <p class="text-sm text-fg-muted">{claimStatusLabels[claim.status]}</p>
  {#if error}<InlineAlert tone="danger">{error}</InlineAlert>{/if}
  {#if notice}<p class="text-sm text-success-fg" role="status">{notice}</p>{/if}
  <label class="grid gap-2 text-sm"
    >处理理由<textarea
      class="min-h-28 rounded-sm border border-line-strong bg-surface p-3"
      bind:value={reason}
      required
      maxlength="2000"></textarea></label
  >
  {#if claim.status === 'PENDING' && claim.method === 'MANUAL'}<div class="flex flex-wrap gap-3">
      <button
        class="min-h-11 rounded-sm bg-primary px-5 font-semibold text-primary-fg disabled:opacity-50"
        type="button"
        disabled={busy || !reason.trim()}
        onclick={() => operate('approve')}>批准认证</button
      ><button
        class="min-h-11 rounded-sm border border-danger px-5 text-danger-fg disabled:opacity-50"
        type="button"
        disabled={busy || !reason.trim()}
        onclick={() => operate('reject')}>驳回认证</button
      >
    </div>{/if}
  <fieldset class="grid gap-4 border-t border-line pt-6">
    <legend class="pt-6 font-semibold">站点归属</legend><label class="grid gap-2 text-sm"
      >处理证据<textarea
        class="min-h-24 rounded-sm border border-line-strong bg-surface p-3"
        bind:value={evidence}
        maxlength="4000"></textarea></label
    ><label class="grid gap-2 text-sm"
      >新所有者用户 ID<input
        class="min-h-11 rounded-sm border border-line-strong bg-surface px-3"
        bind:value={userId}
      /></label
    >
    <div class="flex flex-wrap gap-3">
      <button
        class="min-h-11 rounded-sm border border-line-strong px-4 text-sm disabled:opacity-50"
        type="button"
        disabled={busy || !reason.trim() || !evidence.trim() || !userId.trim()}
        onclick={() => operate('reassign')}>转移归属</button
      ><button
        class="min-h-11 rounded-sm border border-danger px-4 text-sm text-danger-fg disabled:opacity-50"
        type="button"
        disabled={busy || !reason.trim() || !evidence.trim()}
        onclick={() => operate('revoke')}>撤销归属</button
      >
    </div>
  </fieldset>
</section>
