<script lang="ts">
  import { IconDownload, IconUpload } from '@tabler/icons-svelte';
  import { onDestroy, onMount } from 'svelte';

  import { createBackupController } from '../../../application/database-backup/database-backup.controller.svelte.ts';
  import { buttonClass, inputClass, primaryClass } from '../api-keys/api-keys.styles.ts';

  import BackupInspection from './BackupInspection.svelte';

  let { initialError = null }: { initialError?: string | null } = $props();
  const controller = createBackupController();
  let hydrated = $state(false);
  let dialog = $state<HTMLDialogElement>();
  let cancelButton = $state<HTMLButtonElement>();
  let confirmed = $state(false);
  let previousOverflow: string | null = null;
  const loadingLabels = {
    inspect: '正在检查文件…',
    restore: '正在恢复数据，请勿关闭页面…',
  } as const;
  onMount(() => {
    hydrated = true;
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
    releaseScroll();
  });
  function releaseScroll() {
    if (previousOverflow === null) return;
    document.body.style.overflow = previousOverflow;
    previousOverflow = null;
  }
  function confirmRestore() {
    if (!controller.inspection?.can_restore || controller.busy !== null) return;
    confirmed = false;
    previousOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    dialog?.showModal();
    cancelButton?.focus();
  }
  async function restore() {
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
    <form method="POST" action="/management/database-backup/export" data-astro-reload>
      <button type="submit" class={buttonClass} disabled={controller.busy !== null}
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
    <form
      class="mt-5 grid gap-4"
      onsubmit={(event) => {
        event.preventDefault();
        void controller.inspect();
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
        disabled={!hydrated || controller.busy !== null}
        onchange={(event) => controller.select(event.currentTarget.files?.item(0) ?? null)}
      />
      <div class="flex flex-wrap items-center gap-3">
        <button
          type="submit"
          class={buttonClass}
          disabled={!hydrated ||
            !controller.file ||
            controller.file.size === 0 ||
            controller.file.size > 512 * 1024 * 1024 ||
            controller.busy !== null}><IconUpload size={18} aria-hidden="true" />检查文件</button
        >
        {#if controller.busy === 'inspect'}<button
            type="button"
            class={buttonClass}
            onclick={() => controller.cancelInspection()}>取消检查</button
          >{/if}
      </div>
    </form>
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
    {#if controller.inspection}<BackupInspection inspection={controller.inspection} />
      <button
        type="button"
        class="{primaryClass} mt-5"
        disabled={!controller.inspection.can_restore || controller.busy !== null}
        onclick={confirmRestore}>恢复数据</button
      >
    {/if}
  </section>
</div>

<dialog
  bind:this={dialog}
  onclose={releaseScroll}
  class="m-auto max-h-[calc(100dvh-2rem)] w-[min(calc(100%-2rem),30rem)] overflow-y-auto rounded-md border border-line bg-surface p-6 text-fg shadow-md backdrop:bg-overlay"
  aria-labelledby="restore-dialog-title"
  aria-describedby="restore-dialog-description"
>
  <h2 id="restore-dialog-title" class="text-xl font-semibold">确认恢复数据</h2>
  <p id="restore-dialog-description" class="mt-3 text-sm/relaxed text-fg-muted">
    将文件中的全部数据恢复到当前初始化数据库，替换默认种子数据。当前管理员账号和登录资料将保留。
  </p>
  <label class="mt-5 flex min-h-11 items-start gap-3 text-sm/relaxed"
    ><input
      type="checkbox"
      class="mt-1 size-4 shrink-0 accent-primary"
      bind:checked={confirmed}
    />确认将历史管理员操作关联到当前管理员</label
  >
  <div class="mt-6 flex flex-wrap justify-end gap-3">
    <button
      type="button"
      class={buttonClass}
      bind:this={cancelButton}
      onclick={() => dialog?.close()}>取消</button
    >
    <button type="button" class={primaryClass} disabled={!confirmed} onclick={() => void restore()}
      >确认恢复</button
    >
  </div>
</dialog>
