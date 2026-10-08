<script lang="ts">
  import { onDestroy, onMount } from 'svelte';

  import { accountRequest } from '@/api/site-management/site-management.browser';
  import { type FriendLinks, parseFriendLinks } from '@/api/site-management/site-management.types';
  import { managementError, SiteManagementProblem } from '@/application/site-management/messages';
  import type { AccountSubmissionContact } from '@/application/site-submission/site-submission.validation';
  import InlineAlert from '@/components/feedback/InlineAlert.svelte';
  import SiteResolver from '@/components/site-submission/SiteResolver.svelte';
  import SiteSubmissionForm from '@/components/site-submission/SiteSubmissionForm.svelte';
  interface Props {
    shortId: string;
    accountContact: AccountSubmissionContact;
  }
  let { shortId, accountContact }: Props = $props();
  let links = $state.raw<FriendLinks>({ items: [], pending: [] });
  let selected = $state('');
  let creating = $state(false);
  let busy = $state(false);
  let loading = $state(true);
  let error = $state('');
  let notice = $state('');
  const controller = new AbortController();
  onDestroy(() => controller.abort());
  onMount(() => {
    void refresh();
  });
  async function refresh(): Promise<void> {
    loading = true;
    try {
      const response = await accountRequest(`sites/${shortId}/friend-links`, {
        signal: controller.signal,
      });
      if (!response.ok) throw new SiteManagementProblem(await managementError(response));
      const loaded = parseFriendLinks(await response.json());
      links = {
        items: loaded.items.filter((item) => item.link_status === 'ACTIVE'),
        pending: loaded.pending.filter((item) => item.status === 'PENDING'),
      };
    } catch (caught) {
      error =
        caught instanceof SiteManagementProblem ? caught.message : '友链加载失败，请稍后重试。';
    } finally {
      loading = false;
    }
  }
  async function mutate(target: string, operation: 'add' | 'remove' | 'cancel'): Promise<void> {
    if (busy) return;
    if (operation === 'remove' && !confirm('移除这条友链？')) return;
    busy = true;
    error = '';
    notice = '';
    try {
      const suffix =
        operation === 'cancel'
          ? `friend-link-requests/${target}`
          : operation === 'remove'
            ? `friend-links/by-host/${encodeURIComponent(target)}`
            : `friend-links/${target}`;
      const response = await accountRequest(`sites/${shortId}/${suffix}`, {
        method: operation === 'add' ? 'PUT' : 'DELETE',
        signal: controller.signal,
      });
      if (!response.ok) throw new SiteManagementProblem(await managementError(response));
      notice =
        operation === 'add'
          ? '友链已添加。'
          : operation === 'remove'
            ? '友链已移除。'
            : '待建立友链已取消。';
      if (operation === 'add') {
        selected = '';
        await refresh();
      } else if (operation === 'remove')
        links = { ...links, items: links.items.filter((item) => item.target_host !== target) };
      else links = { ...links, pending: links.pending.filter((item) => item.id !== target) };
    } catch (caught) {
      error =
        caught instanceof SiteManagementProblem ? caught.message : '友链操作失败，请稍后重试。';
    } finally {
      busy = false;
    }
  }
</script>

<section class="grid gap-6" aria-labelledby="friend-heading">
  <h2 class="text-xl font-semibold" id="friend-heading">友链管理</h2>
  {#if error}<InlineAlert tone="danger">{error}</InlineAlert>{/if}
  {#if notice}<p class="text-sm text-success-fg" role="status">{notice}</p>{/if}
  {#if loading}<p class="text-sm text-fg-muted" role="status">正在加载友链…</p>{:else}
    {#if links.items.length === 0}<p class="text-sm text-fg-muted">暂无友链。</p>{/if}
    <ul class="divide-y divide-line">
      {#each links.items as link (link.target_host)}<li
          class="flex flex-wrap items-center justify-between gap-4 py-4"
        >
          <div class="min-w-0">
            {#if link.target_short_id}<a
                class="font-medium text-tint-fg"
                href={`/site/${link.target_short_id}`}>{link.target_name}</a
              >
            {:else}<strong class="font-medium">{link.target_host}</strong>
              <p class="text-xs text-fg-muted">暂无可查看的站点资料</p>{/if}
            <p class="text-sm break-all text-fg-muted">{link.target_url}</p>
            <p class="text-xs text-fg-muted">{link.is_reciprocal ? '双向友链' : '单向友链'}</p>
          </div>
          <button
            class="min-h-11 rounded-sm border border-danger px-4 text-sm text-danger-fg"
            type="button"
            disabled={busy}
            onclick={() => mutate(link.target_host, 'remove')}>移除</button
          >
        </li>{/each}
    </ul>
    {#if links.pending.length}<h3 class="font-semibold">待建立友链</h3>
      <ul class="divide-y divide-line">
        {#each links.pending as item (item.id)}<li class="grid gap-3 py-4">
            <strong class="text-sm">{item.target_name}</strong>
            <p class="text-sm break-all text-fg-muted">{item.target_url}</p>
            <div class="flex flex-wrap gap-4">
              <a
                class="inline-flex min-h-11 items-center text-sm text-tint-fg underline"
                href={`/dashboard/submissions/${item.audit_id}`}>查看审核进度</a
              ><button
                class="min-h-11 rounded-sm border border-line-strong px-4 text-sm"
                type="button"
                disabled={busy}
                onclick={() => mutate(item.id, 'cancel')}>取消建立友链</button
              >
            </div>
          </li>{/each}
      </ul>{/if}
  {/if}
  <div class="grid gap-4 border-t border-line pt-6">
    <SiteResolver
      initialQuery=""
      resolving={busy}
      onresolve={async (id) => {
        selected = id;
      }}
    />
    {#if selected}<p class="text-sm text-fg-muted">所选站点：{selected}</p>
      <button
        class="min-h-11 w-fit rounded-sm bg-primary px-5 font-semibold text-primary-fg disabled:opacity-50"
        type="button"
        disabled={busy || selected === shortId}
        onclick={() => mutate(selected, 'add')}>添加友链</button
      >{/if}
    <button
      class="min-h-11 w-fit rounded-sm border border-line-strong px-4 text-sm"
      type="button"
      onclick={() => (creating = !creating)}
      >{creating ? '收起新增申请' : '未收录站点，提交新增申请'}</button
    >
    {#if creating}<SiteSubmissionForm
        action="CREATE"
        accountEndpoint={`sites/${shortId}/friend-link-submissions`}
        {accountContact}
      />{/if}
  </div>
</section>
