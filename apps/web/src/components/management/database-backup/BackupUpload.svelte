<script lang="ts">
  import { IconUpload } from '@tabler/icons-svelte';

  import type { BackupUploadController } from '../../../application/database-backup/database-backup.upload-controller.svelte.ts';
  import { buttonClass, inputClass } from '../api-keys/api-keys.styles.ts';

  let {
    upload,
    disabled,
    canDiscard,
    start,
  }: { upload: BackupUploadController; disabled: boolean; canDiscard: boolean; start: () => void } =
    $props();
  const labels = {
    uploading: '文件等待上传',
    preparing: '正在准备文件',
    preparation_failed: '文件准备未完成',
    ready: '文件已准备好',
    inspecting: '正在检查文件',
    inspected: '文件检查完成',
    restoring: '正在恢复数据',
    succeeded: '恢复完成',
    failed: '操作未完成',
    cancelled: '已放弃文件',
    expired: '文件已过期',
    unknown: '恢复结果待确认',
  } as const;
  const stage = $derived(
    upload.busy === 'hashing'
      ? '正在核对文件'
      : upload.busy === 'uploading'
        ? '正在上传文件'
        : upload.busy === 'inspecting'
          ? '正在检查文件'
          : upload.session
            ? labels[upload.session.state]
            : '',
  );
  const preparing = $derived(upload.session?.state === 'preparing');
  const total = $derived(upload.session?.size ?? upload.file?.size ?? 0);
  const bytes = $derived(preparing ? (upload.session?.prepared_bytes ?? 0) : upload.bytes);
  const percent = $derived(total ? Math.min(100, (bytes / total) * 100) : 0);
  const resumable = $derived(upload.session?.state === 'uploading');
  const newUpload = $derived(!upload.session || upload.session.cleanup_status === 'completed');
  function size(value: number) {
    return `${(value / 1024 / 1024).toFixed(1)} MiB`;
  }
  function speed(value: number) {
    return value >= 1024 * 1024
      ? `${(value / 1024 / 1024).toFixed(1)} MiB/s`
      : `${(value / 1024).toFixed(0)} KiB/s`;
  }
</script>

<form
  class="mt-5 grid gap-4"
  onsubmit={(event) => {
    event.preventDefault();
    start();
  }}
>
  <label for="backup-file" class="text-sm font-medium"
    >JSON 文件 <span class="font-normal text-fg-muted">（最大 512 MiB）</span></label
  >
  <input
    id="backup-file"
    type="file"
    accept=".json,application/json"
    class="{inputClass} py-2 file:mr-3 file:rounded-sm file:border-0 file:bg-subtle file:px-3 file:py-1 file:text-sm file:text-fg"
    disabled={disabled || !upload.settings?.enabled || (!resumable && !newUpload)}
    onchange={(event) => upload.select(event.currentTarget.files?.item(0) ?? null)}
  />
  {#if upload.settings && !upload.settings.enabled}<p class="text-sm text-fg-muted" role="status">
      文件上传暂未启用，仍可导出数据。
    </p>{/if}
  {#if resumable && !upload.file}<p class="text-sm text-fg-muted">
      请选择之前的同一文件，继续上传。
    </p>{/if}
  <div class="flex flex-wrap items-center gap-3">
    {#if resumable || newUpload}
      <button
        type="submit"
        class={buttonClass}
        disabled={disabled ||
          !upload.settings?.enabled ||
          !upload.file ||
          upload.file.size === 0 ||
          upload.file.size > 512 * 1024 * 1024}
      >
        <IconUpload size={18} aria-hidden="true" />{resumable ? '继续上传' : '上传文件'}
      </button>
    {/if}
    {#if upload.busy === 'uploading' || upload.busy === 'hashing'}<button
        type="button"
        class={buttonClass}
        onclick={() => upload.pause()}>暂停上传</button
      >{/if}
    {#if upload.session?.state === 'preparation_failed'}<button
        type="button"
        class={buttonClass}
        {disabled}
        onclick={() => void upload.prepare()}>重新准备文件</button
      >{/if}
    {#if upload.usable}<button
        type="button"
        class={buttonClass}
        {disabled}
        onclick={() => void upload.inspect()}>重新检查</button
      >{/if}
    {#if upload.session && !newUpload && upload.session.state !== 'restoring' && upload.session.state !== 'unknown'}<button
        type="button"
        class={buttonClass}
        disabled={!canDiscard}
        onclick={() => void upload.discard()}>放弃文件</button
      >{/if}
  </div>
</form>
{#if stage}<div class="mt-5 grid gap-3 rounded-md border border-line p-4">
    <p class="text-sm font-medium" role="status">{stage}</p>
    {#if upload.busy === 'uploading' || resumable || preparing}
      <div
        class="h-1.5 overflow-hidden rounded-full bg-subtle"
        role="progressbar"
        aria-label={preparing ? '服务器接收进度' : '文件上传进度'}
        aria-valuemin="0"
        aria-valuemax={total}
        aria-valuenow={bytes}
      >
        <div
          class="h-full origin-left bg-primary transition-transform duration-400 ease-[cubic-bezier(0.4,0,0.2,1)] motion-reduce:transition-none"
          style:transform="scaleX({percent / 100})"
        ></div>
      </div>
      <p class="text-sm text-fg-muted tabular-nums">
        {size(bytes)} / {size(total)} · {percent.toFixed(1)}%{#if upload.busy === 'uploading'}
          · {speed(upload.speed)}{/if}
      </p>
    {/if}
    {#if preparing}<p class="text-xs text-fg-muted">
        文件上传完成，正在接收并核对文件。完成后自动检查。
      </p>{/if}
  </div>{/if}
{#if upload.session?.cleanup_status === 'pending'}<p
    class="mt-4 text-sm text-fg-muted"
    role="status"
  >
    临时文件正在清理，系统会自动重试。恢复结果不受影响。
  </p>{/if}
{#if upload.error}<p class="mt-4 rounded-md bg-danger-bg p-4 text-sm text-danger-fg" role="alert">
    {upload.error}
  </p>{/if}
