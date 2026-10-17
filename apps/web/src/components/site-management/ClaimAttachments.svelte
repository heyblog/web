<script lang="ts">
  import type { ClaimAttachment } from '@/api/site-management/claims.types';
  import {
    claimAttachmentLimits,
    claimAttachmentTotal,
  } from '@/application/site-management/claim-attachments';
  import { buttonClass, inputClass } from '@/components/management/api-keys/api-keys.styles';
  let {
    attachments,
    busy,
    error,
    onadd,
    onremove,
  }: {
    attachments: readonly ClaimAttachment[];
    busy: boolean;
    error: string;
    onadd: (files: readonly File[]) => Promise<void>;
    onremove: (id: string) => void;
  } = $props();
  function selectFiles(event: Event) {
    const target = event.currentTarget;
    if (!(target instanceof HTMLInputElement) || !target.files) return;
    void onadd(Array.from(target.files));
    target.value = '';
  }
  function size(bytes: number): string {
    return `${(bytes / (1024 * 1024)).toFixed(bytes >= 1024 * 1024 ? 1 : 2)} MiB`;
  }
</script>

<div class="grid gap-3">
  <label class="grid gap-2 text-sm">
    <span class="font-medium">证明附件（可选）</span>
    <input
      class={inputClass}
      type="file"
      accept=".png,.jpg,.jpeg,.pdf,image/png,image/jpeg,application/pdf"
      multiple
      disabled={busy || attachments.length >= claimAttachmentLimits.maxCount}
      onchange={selectFiles}
    />
  </label>
  <p class="text-xs/5 text-fg-muted">
    支持 PNG、JPEG 和 PDF；图片单个不超过 5 MiB，PDF 单个不超过 10 MiB，最多 3 个，合计不超过 20
    MiB。
  </p>
  {#if error}<p class="rounded-md bg-danger-bg p-3 text-sm text-danger-fg" role="alert">
      {error}
    </p>{/if}
  {#if attachments.length}
    <ul class="grid gap-2" aria-label="已上传证明附件">
      {#each attachments as attachment (attachment.id)}
        <li
          class="flex min-h-11 items-center justify-between gap-3 rounded-md border border-line p-3 text-sm"
        >
          <span class="min-w-0"
            ><span class="block truncate font-medium">{attachment.filename}</span><span
              class="block text-xs text-fg-muted"
              >{attachment.content_type} · {size(attachment.size)}</span
            ></span
          >
          <button
            class={buttonClass}
            type="button"
            disabled={busy}
            onclick={() => onremove(attachment.id)}>移除</button
          >
        </li>
      {/each}
    </ul>
    <p class="text-xs text-fg-muted">
      已上传 {attachments.length}/{claimAttachmentLimits.maxCount} 个，合计 {size(
        claimAttachmentTotal(attachments),
      )}。
    </p>
  {/if}
  {#if busy}<p class="text-sm text-fg-muted" role="status">正在上传证明附件…</p>{/if}
</div>
