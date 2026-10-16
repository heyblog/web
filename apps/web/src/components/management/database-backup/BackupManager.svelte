<script lang="ts">
  import { IconDownload } from '@tabler/icons-svelte';
  import { onDestroy, onMount } from 'svelte';

  import type { CurrentBackupUpload } from '../../../api/database-backup/database-backup.uploads.ts';
  import { createBackupController } from '../../../application/database-backup/database-backup.controller.svelte.ts';
  import { buttonClass, primaryClass } from '../api-keys/api-keys.styles.ts';

  import BackupInspection from './BackupInspection.svelte';
  import BackupProgress from './BackupProgress.svelte';
  import BackupUpload from './BackupUpload.svelte';

  let {
    initialError = null,
    initialUploads = null,
  }: { initialError?: string | null; initialUploads?: CurrentBackupUpload | null } = $props();
  const controller = createBackupController(undefined, initialUploads);
  let hydrated = $state(false);
  let dialog = $state<HTMLDialogElement>();
  let cancelButton = $state<HTMLButtonElement>();
  let confirmed = $state(false);
  let confirmation = $state<'restore' | 'cancel'>('restore');
  const loadingLabels = {
    restore: '正在提交恢复任务…',
    cancel: '正在请求取消恢复…',
  } as const;
  onMount(() => {
    hydrated = true;
    void controller.refresh();
    const preventLeaving = (event: BeforeUnloadEvent) => {
      if (controller.busy !== 'restore') return;
      event.preventDefault();
      event.returnValue = '';
    };
    window.addEventListener('beforeunload', preventLeaving);
    return () => window.removeEventListener('beforeunload', preventLeaving);
  });
  onDestroy(() => {
    controller.dispose();
    dialog?.close();
  });
  function confirmRestore() {
    if (!controller.upload.inspection?.can_restore || controller.blocked) return;
    confirmation = 'restore';
    confirmed = false;
    dialog?.showModal();
    cancelButton?.focus();
  }
  async function restore() {
    if (confirmation === 'cancel') {
      dialog?.close();
      await controller.cancel();
      return;
    }
    if (!confirmed) return;
    dialog?.close();
    await controller.restore();
  }
</script>

<div class="grid max-w-4xl gap-8">
  {#if initialError}<p class="rounded-md bg-danger-bg p-4 text-sm text-danger-fg" role="alert">
      {initialError}
    </p>{/if}
  <section class="grid gap-4 border-b border-line pb-8" aria-labelledby="backup-export-heading">
    <div>
      <h2 id="backup-export-heading" class="text-xl font-semibold">导出数据</h2>
      <p class="mt-2 text-sm/relaxed text-fg-muted">
        下载全部业务数据、审计记录和关系。系统管理员账号不包含在文件中。
      </p>
    </div>
    <form method="POST" action="/api/web/management/database-backup/export" data-astro-reload>
      <button type="submit" class={buttonClass} disabled={controller.blocked}
        ><IconDownload size={18} aria-hidden="true" />导出 JSON</button
      >
    </form>
    <p class="text-xs/relaxed text-fg-muted">文件包含个人信息和凭证验证资料，请妥善保存。</p>
  </section>
  <section aria-labelledby="backup-import-heading" aria-busy={controller.busy !== null}>
    <h2 id="backup-import-heading" class="text-xl font-semibold">导入数据</h2>
    <p class="mt-2 text-sm/relaxed text-fg-muted">
      恢复到仅配置系统管理员的新数据库。业务 ID 和凭证标识保持不变。
    </p>
    <BackupUpload
      upload={controller.upload}
      canDiscard={hydrated &&
        !controller.active &&
        !controller.busy &&
        !controller.checking &&
        !controller.acceptanceUnknown &&
        !controller.upload.busy}
      disabled={!hydrated || controller.blocked}
      start={() => {
        void controller.startUpload();
      }}
    />
    {#if controller.checking}<p class="mt-4 text-sm text-fg-muted" role="status">
        正在查询文件和恢复任务…
      </p>{/if}
    {#if controller.job}<BackupProgress job={controller.job} />{/if}
    {#if controller.job?.status === 'running'}
      <button
        type="button"
        class="{buttonClass} mt-4"
        disabled={controller.busy !== null || controller.job.progress.stage === 'committing'}
        onclick={() => {
          confirmation = 'cancel';
          dialog?.showModal();
          cancelButton?.focus();
        }}>取消恢复</button
      >
    {/if}
    {#if controller.acceptanceUnknown}<button
        type="button"
        class="{buttonClass} mt-4"
        disabled={controller.checking}
        onclick={() => void controller.refresh()}>查询任务状态</button
      >{/if}
    {#if controller.busy}<p class="mt-4 text-sm text-fg-muted" role="status">
        {loadingLabels[controller.busy]}
      </p>{/if}
    {#if controller.error}<p
        class="mt-4 rounded-md bg-danger-bg p-4 text-sm text-danger-fg"
        role="alert"
      >
        {controller.error}
      </p>{/if}
    {#if controller.restoration}<div
        class="mt-5 grid gap-3 rounded-md bg-success-bg p-4 text-sm text-success-fg"
        role="status"
      >
        <p class="font-medium">恢复完成。业务 ID、凭证和历史关联已保留。</p>
        <p>管理员关联已迁移到当前账号。</p>
        <a
          href="/dashboard"
          class="inline-flex min-h-11 w-fit items-center font-medium underline underline-offset-4"
          >返回工作台</a
        >
      </div>{/if}
    {#if controller.upload.inspection}<BackupInspection inspection={controller.upload.inspection} />
      <button
        type="button"
        class="{primaryClass} mt-5"
        disabled={!controller.upload.inspection.can_restore || controller.blocked}
        onclick={confirmRestore}>恢复数据</button
      >
    {/if}
  </section>
</div>

<dialog
  bind:this={dialog}
  class="m-auto max-h-[calc(100dvh-2rem)] w-[min(calc(100%-2rem),30rem)] overflow-y-auto rounded-md border border-line bg-surface p-6 text-fg shadow-md backdrop:bg-overlay"
  aria-labelledby="restore-dialog-title"
  aria-describedby="restore-dialog-description"
>
  <h2 id="restore-dialog-title" class="text-xl font-semibold">
    {confirmation === 'restore' ? '确认恢复数据' : '取消恢复？'}
  </h2>
  <p id="restore-dialog-description" class="mt-3 text-sm/relaxed text-fg-muted">
    {confirmation === 'restore'
      ? '将文件中的全部数据恢复到当前初始化数据库，替换默认种子数据。当前管理员账号和登录资料将保留。'
      : '取消后将回滚本次恢复。请等待回滚结果确认。'}
  </p>
  {#if confirmation === 'restore'}<label
      class="mt-5 flex min-h-11 items-start gap-3 text-sm/relaxed"
      ><input
        type="checkbox"
        class="mt-1 size-4 shrink-0 accent-primary"
        bind:checked={confirmed}
      />确认将历史管理员操作关联到当前管理员</label
    >
  {/if}
  <div class="mt-6 flex flex-wrap justify-end gap-3">
    <button
      type="button"
      class={buttonClass}
      bind:this={cancelButton}
      onclick={() => dialog?.close()}>取消</button
    >
    <button
      type="button"
      class={primaryClass}
      disabled={confirmation === 'restore' && !confirmed}
      onclick={() => void restore()}>{confirmation === 'restore' ? '确认恢复' : '确认取消'}</button
    >
  </div>
</dialog>
