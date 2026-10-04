<script lang="ts">
  import { IconExternalLink, IconSpeakerphone, IconX } from '@tabler/icons-svelte';
  import { onMount } from 'svelte';

  import type { PublicAnnouncement } from '../../api/announcements/announcements.types.ts';
  import {
    dismissAnnouncement,
    isAnnouncementDismissed,
  } from '../../application/announcements/announcement-dismissal.browser.ts';

  import AnnouncementInline from './AnnouncementInline.svelte';

  let { announcement }: { announcement: PublicAnnouncement } = $props();
  let ready = $state(false);
  let visible = $derived(ready && !isAnnouncementDismissed(announcement.id));

  onMount(() => {
    ready = true;
  });
</script>

{#if visible}
  <aside class="border-b border-line bg-tint" aria-label="横幅公告">
    <div
      class="mx-auto flex w-[min(calc(100%-2rem),80rem)] flex-wrap items-center gap-x-4 gap-y-2 py-3 text-sm sm:w-[min(calc(100%-3rem),80rem)]"
    >
      <IconSpeakerphone class="shrink-0 text-tint-fg" size={18} aria-hidden="true" />
      <div class="min-w-0 flex-1 leading-6 wrap-anywhere">
        <strong class="font-medium">{announcement.title}</strong>
        {#if announcement.bodyMarkdown}
          <span class="ml-2 text-fg-muted">
            <AnnouncementInline source={announcement.bodyMarkdown} />
          </span>
        {/if}
      </div>
      {#if announcement.action}
        <a
          class="inline-flex min-h-11 items-center gap-1 font-medium text-tint-fg sm:min-h-10"
          href={announcement.action.href}
          rel={announcement.action.external ? 'noopener noreferrer' : undefined}
          target={announcement.action.external ? '_blank' : undefined}
        >
          {announcement.action.label}
          {#if announcement.action.external}<IconExternalLink size={16} aria-hidden="true" />{/if}
        </a>
      {/if}
      <button
        type="button"
        class="inline-flex size-11 shrink-0 items-center justify-center rounded-md text-fg-muted hover:bg-subtle hover:text-fg sm:size-10"
        aria-label="关闭公告"
        title="关闭公告"
        onclick={() => {
          dismissAnnouncement(announcement.id);
          visible = false;
        }}><IconX size={18} aria-hidden="true" /></button
      >
    </div>
  </aside>
{/if}
