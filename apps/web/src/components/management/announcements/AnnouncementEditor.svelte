<script lang="ts">
  import {
    IconArchive,
    IconArrowLeft,
    IconDeviceFloppy,
    IconEye,
    IconRefresh,
    IconSend,
    IconTrash,
  } from '@tabler/icons-svelte';
  import { onMount, tick, untrack } from 'svelte';

  import type {
    AnnouncementRevision,
    ManagedAnnouncement,
  } from '../../../api/announcements/announcements.types.ts';
  import { statusLabels } from '../../../application/announcements/announcement-editor.shared.ts';
  import { createAnnouncementEditor } from '../../../application/announcements/announcement-editor.svelte.ts';
  import AnnouncementInline from '../../announcements/AnnouncementInline.svelte';

  import AnnouncementFields from './AnnouncementFields.svelte';
  import AnnouncementRevisions from './AnnouncementRevisions.svelte';

  let {
    initialAnnouncement,
    initialRevisions = [],
  }: {
    initialAnnouncement: ManagedAnnouncement | null;
    initialRevisions?: readonly AnnouncementRevision[] | null;
  } = $props();
  const editor = untrack(() => createAnnouncementEditor(initialAnnouncement, initialRevisions));
  let preview = $state(false);
  let ready = $state(false);
  let timeZone = $state('');
  let form = $state<HTMLFormElement>();
  const archived = $derived(editor.announcement?.status === 'ARCHIVED');
  const identityLocked = $derived(
    editor.announcement?.status === 'PUBLISHED' &&
      editor.announcement.startsAt !== null &&
      Date.parse(editor.announcement.startsAt) <= Date.now(),
  );
  const buttonClass =
    'inline-flex min-h-11 items-center justify-center gap-2 rounded-md border border-line-strong bg-surface px-4 text-sm font-medium transition-colors duration-(--motion-color) hover:bg-subtle disabled:cursor-not-allowed disabled:opacity-50 sm:min-h-10';

  onMount(() => {
    ready = true;
    timeZone = Intl.DateTimeFormat().resolvedOptions().timeZone;
    const beforeUnload = (event: BeforeUnloadEvent) => {
      if (editor.dirty) event.preventDefault();
    };
    const leaving = (event: MouseEvent) => {
      const target = event.target;
      const anchor = target instanceof Element ? target.closest('a[href]') : null;
      if (
        !anchor ||
        !editor.dirty ||
        event.defaultPrevented ||
        event.button !== 0 ||
        event.ctrlKey ||
        event.metaKey ||
        event.shiftKey ||
        event.altKey
      )
        return;
      if (!window.confirm('放弃未保存的修改？')) event.preventDefault();
    };
    window.addEventListener('beforeunload', beforeUnload);
    document.addEventListener('click', leaving);
    return () => {
      window.removeEventListener('beforeunload', beforeUnload);
      document.removeEventListener('click', leaving);
    };
  });
  async function submit(operation: () => Promise<void>) {
    await operation();
    await tick();
    form?.querySelector<HTMLElement>('[aria-invalid="true"]:not(:disabled)')?.focus();
  }
</script>

<div class="max-w-4xl">
  <div class="mb-6 flex flex-wrap items-center justify-between gap-3">
    <a
      class="inline-flex min-h-11 items-center gap-2 text-sm text-tint-fg"
      href="/management/announcements"><IconArrowLeft size={16} aria-hidden="true" />公告列表</a
    >
    {#if editor.announcement}<span
        class="rounded-sm bg-tint px-2 py-1 text-xs font-medium text-tint-fg"
        >{statusLabels[editor.announcement.effectiveStatus]}</span
      >{/if}
  </div>
  {#if editor.error}<div
      class="mb-6 rounded-md border border-danger bg-danger-bg p-4 text-sm text-danger-fg"
      role="alert"
    >
      <p>{editor.error}</p>
      {#if editor.conflict}<button
          class="mt-3 inline-flex min-h-11 items-center gap-2 font-medium"
          type="button"
          disabled={editor.busy}
          onclick={() => {
            if (!editor.dirty || window.confirm('放弃未保存的修改并加载最新版本？'))
              void editor.reload();
          }}><IconRefresh size={16} aria-hidden="true" />重新加载</button
        >{/if}
    </div>{/if}
  {#if editor.notice}<p class="mb-6 text-sm text-success-fg" role="status">{editor.notice}</p>{/if}
  <form
    bind:this={form}
    novalidate
    aria-busy={editor.busy}
    onsubmit={(event) => {
      event.preventDefault();
      void submit(editor.save);
    }}
  >
    <AnnouncementFields
      bind:draft={editor.draft}
      {identityLocked}
      disabled={!ready || editor.busy || archived}
      {timeZone}
      fieldErrors={editor.fieldErrors}
      attempted={editor.attempted}
    />
    <div class="mt-6 flex flex-wrap items-center gap-3 border-t border-line pt-6">
      {#if !archived}<button
          type="submit"
          class="inline-flex min-h-11 items-center gap-2 rounded-md bg-primary px-4 text-sm font-medium text-primary-fg hover:bg-primary-hover disabled:cursor-not-allowed disabled:opacity-50 sm:min-h-10"
          disabled={!ready || editor.busy}
          ><IconDeviceFloppy size={16} aria-hidden="true" />{editor.busy
            ? '处理中'
            : '保存'}</button
        >{/if}
      <button
        type="button"
        class={buttonClass}
        aria-pressed={preview}
        onclick={() => {
          preview = !preview;
        }}><IconEye size={16} aria-hidden="true" />预览</button
      >
      {#if editor.announcement?.status === 'DRAFT'}
        <button
          type="button"
          class={buttonClass}
          disabled={!ready || editor.busy || editor.dirty}
          onclick={() => {
            void submit(() => editor.publish(false));
          }}><IconSend size={16} aria-hidden="true" />立即发布</button
        >
        <button
          type="button"
          class={buttonClass}
          disabled={!ready || editor.busy || editor.dirty}
          onclick={() => {
            void submit(() => editor.publish(true));
          }}>定时发布</button
        >
        <button
          type="button"
          class={`${buttonClass} text-danger-fg`}
          disabled={!ready || editor.busy}
          onclick={() => {
            if (window.confirm('删除这条草稿？未保存的修改也将丢弃。')) void editor.delete();
          }}><IconTrash size={16} aria-hidden="true" />删除草稿</button
        >
      {:else if editor.announcement?.status === 'PUBLISHED'}
        <button
          type="button"
          class={buttonClass}
          disabled={!ready || editor.busy || editor.dirty}
          onclick={() => {
            if (window.confirm('归档这条公告？归档后无法编辑或重新发布。')) void editor.archive();
          }}><IconArchive size={16} aria-hidden="true" />归档</button
        >
      {/if}
      {#if editor.dirty}<span class="text-xs text-fg-muted">有未保存的修改</span>{/if}
    </div>
  </form>
  {#if preview}
    <section class="mt-8 border-t border-line pt-6" aria-label="公告预览">
      <h2 class="mb-4 text-sm font-medium text-fg-muted">预览</h2>
      <article class="rounded-md border border-line bg-surface p-6 shadow-2xs">
        <h3 class="text-lg font-semibold wrap-anywhere">{editor.draft.title}</h3>
        <div class="mt-3 text-sm/6">
          <AnnouncementInline source={editor.draft.bodyMarkdown} />
        </div>
      </article>
    </section>
  {/if}
  {#if editor.announcement}<AnnouncementRevisions revisions={editor.revisions} />{/if}
</div>
