<script lang="ts">
  import { onDestroy } from 'svelte';

  import type { OwnershipDetail } from '@/api/site-management/claims.types';
  import { formFeedback } from '@/application/forms/field-feedback.browser';
  import {
    buttonClass,
    dangerClass,
    inputClass,
  } from '@/components/management/api-keys/api-keys.styles';
  let {
    ownership,
    busy,
    onrevoke,
  }: {
    ownership: OwnershipDetail;
    busy: boolean;
    onrevoke: (input: { readonly reason: string; readonly evidence: string }) => Promise<void>;
  } = $props();
  let reason = $state('');
  let evidence = $state('');
  let dialog = $state<HTMLDialogElement>();
  let dismiss = $state<HTMLButtonElement>();
  const actions: Readonly<Record<string, string>> = {
    VERIFIED: '验证通过',
    REVOKED: '归属已撤销',
    REASSIGNED: '归属已调整',
  };
  onDestroy(() => dialog?.close());
</script>

<section class="grid gap-4 border-t border-line pt-6" aria-labelledby="ownership-title">
  <h2 id="ownership-title" class="text-lg font-semibold">当前站点归属</h2>
  {#if ownership.current}
    <div class="grid gap-2 rounded-md bg-subtle p-4">
      <p class="text-sm font-medium">
        {ownership.current.owner.display_name}
        <span class="text-fg-muted">@{ownership.current.owner.username}</span>
      </p>
      <p class="text-sm break-all text-fg-muted">{ownership.current.address}</p>
      {#if !ownership.current.is_current}<p class="text-sm text-warning-fg">
          当前主页地址尚待重新验证。
        </p>{/if}
    </div>
    {#if ownership.can_revoke}<button
        class="{dangerClass} w-fit"
        type="button"
        disabled={busy}
        onclick={() => {
          reason = '';
          evidence = '';
          dialog?.showModal();
          dismiss?.focus();
        }}>撤销站点归属</button
      >{/if}
  {:else}<p class="text-sm text-fg-muted">此站点当前没有所有者。</p>{/if}
  {#if ownership.history.length}<details class="rounded-md border border-line p-4">
      <summary class="cursor-pointer text-sm font-medium">归属处理记录</summary>
      <ol class="mt-3 divide-y divide-line">
        {#each ownership.history as event (event.id)}<li class="grid gap-2 py-3">
            <p class="text-sm font-medium">{actions[event.action] ?? '归属处理'}</p>
            <time class="text-xs text-fg-muted" datetime={event.created_at}
              >{new Date(event.created_at).toLocaleString('zh-CN')}</time
            >
            <p class="text-sm whitespace-pre-wrap">{event.reason}</p>
            {#if event.evidence}<p class="text-sm whitespace-pre-wrap text-fg-muted">
                {event.evidence}
              </p>{/if}
          </li>{/each}
      </ol>
    </details>{/if}
</section>
<dialog
  bind:this={dialog}
  class="m-auto max-h-[calc(100dvh-2rem)] w-[min(calc(100%-2rem),30rem)] overflow-y-auto rounded-md border border-line bg-surface p-6 text-fg shadow-md backdrop:bg-overlay"
  aria-labelledby="revoke-title"
  aria-describedby="revoke-description"
>
  <form
    use:formFeedback
    class="grid gap-4"
    onsubmit={(event) => {
      event.preventDefault();
      dialog?.close();
      void onrevoke({ reason, evidence });
    }}
  >
    <h2 id="revoke-title" class="text-xl font-semibold">撤销站点归属？</h2>
    <p id="revoke-description" class="text-sm/relaxed text-fg-muted">
      {ownership.current?.owner.display_name} 将失去此站点的管理权限。站点资料和历史记录将保留。
    </p>
    <label class="grid gap-2 text-sm"
      >处理理由<textarea
        class="{inputClass} min-h-24 py-3"
        bind:value={reason}
        required
        name="reason"
        data-max-bytes="2000"
        data-field-label="处理理由"
        aria-describedby="reason-requirements"></textarea><span
        id="reason-requirements"
        class="text-xs/normal text-fg-muted">必填，最多 2000 字节。</span
      ></label
    ><label class="grid gap-2 text-sm"
      >处理证据<textarea
        class="{inputClass} min-h-24 py-3"
        bind:value={evidence}
        required
        name="evidence"
        data-max-bytes="4000"
        data-field-label="归属说明"
        aria-describedby="evidence-requirements"></textarea><span
        id="evidence-requirements"
        class="text-xs/normal text-fg-muted">必填，最多 4000 字节。</span
      ></label
    >
    <div class="flex flex-wrap justify-end gap-3">
      <button bind:this={dismiss} class={buttonClass} type="button" onclick={() => dialog?.close()}
        >返回</button
      ><button
        class={dangerClass}
        type="submit"
        disabled={busy || !reason.trim() || !evidence.trim()}>确认撤销</button
      >
    </div>
  </form>
</dialog>
