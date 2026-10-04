<script lang="ts">
  import {
    IconArrowLeft,
    IconArrowRight,
    IconExternalLink,
    IconPlayerPause,
    IconPlayerPlay,
  } from '@tabler/icons-svelte';
  import { onMount } from 'svelte';

  import type { PublicAnnouncement } from '../../api/announcements/announcements.types.ts';
  import { createAnnouncementPlayback } from '../../application/announcements/announcement-playback.browser.ts';
  import AnnouncementInline from '../announcements/AnnouncementInline.svelte';

  let { announcements }: { announcements: readonly PublicAnnouncement[] } = $props();
  let current = $state(0);
  const activeAnnouncement = $derived(announcements[current]);
  let ready = $state(false);
  let userPaused = $state(false);
  let root = $state<HTMLElement>();
  let playback: ReturnType<typeof createAnnouncementPlayback> | undefined;
  const controlClass =
    'inline-flex size-11 shrink-0 items-center justify-center rounded-md text-fg-muted transition-colors duration-(--motion-color) hover:bg-subtle hover:text-fg sm:size-10';

  onMount(() => {
    if (!root) return;
    const reduced = window.matchMedia('(prefers-reduced-motion: reduce)');
    playback = createAnnouncementPlayback(announcements.length, (index) => {
      current = index;
    });
    userPaused = reduced.matches;
    playback.pause('user', userPaused);
    const visibility = () => playback?.pause('hidden', document.hidden);
    const motion = () => {
      if (reduced.matches) {
        userPaused = true;
        playback?.pause('user', true);
      }
    };
    visibility();
    playback.pause('focus', root.contains(document.activeElement));
    playback.pause('hover', root.matches(':hover') && window.matchMedia('(hover: hover)').matches);
    document.addEventListener('visibilitychange', visibility);
    reduced.addEventListener('change', motion);
    ready = true;
    playback.start();
    return () => {
      playback?.dispose();
      document.removeEventListener('visibilitychange', visibility);
      reduced.removeEventListener('change', motion);
    };
  });

  function togglePlayback() {
    userPaused = !userPaused;
    playback?.pause('user', userPaused);
  }
</script>

{#if announcements.length > 0}
  <section
    bind:this={root}
    class="min-w-0"
    aria-label="站点公告"
    aria-roledescription="轮播"
    onmouseenter={() => playback?.pause('hover', true)}
    onmouseleave={() => playback?.pause('hover', false)}
    onfocusin={() => playback?.pause('focus', true)}
    onfocusout={(event) => {
      if (!(event.relatedTarget instanceof Node) || !root?.contains(event.relatedTarget))
        playback?.pause('focus', false);
    }}
  >
    <div class="grid" aria-live="off">
      {#each announcements as announcement, index (announcement.id)}
        <article
          class={`col-start-1 row-start-1 flex min-w-0 flex-col transition-opacity duration-(--motion-base) ease-standard motion-reduce:transition-none ${current === index ? 'visible opacity-100' : 'invisible opacity-0'}`}
          inert={current !== index}
          aria-hidden={current !== index}
          aria-roledescription="幻灯片"
          aria-label={`${index + 1} / ${announcements.length}`}
        >
          <div
            class="flex flex-wrap items-baseline gap-x-3 gap-y-1 font-mono text-xs text-fg-muted"
          >
            <p class="font-medium">站点公告</p>
            <time datetime={announcement.startsAt}
              >{new Date(announcement.startsAt).toLocaleDateString('zh-CN', {
                timeZone: 'Asia/Shanghai',
              })}</time
            >
          </div>
          <h2 class="mt-2 text-lg/7 font-semibold wrap-anywhere">
            <a class="hover:text-tint-fg" href={`/announcements/${announcement.id}`}
              ><span class="line-clamp-2">{announcement.title}</span></a
            >
          </h2>
          {#if announcement.bodyMarkdown}
            <div class="mt-3 text-sm/6 text-fg-muted">
              <div class="line-clamp-3">
                <AnnouncementInline source={announcement.bodyMarkdown} interactiveLinks={false} />
              </div>
            </div>
          {/if}
        </article>
      {/each}
    </div>
    <div class="mt-4 grid items-center gap-x-4 gap-y-2 sm:grid-cols-[auto_minmax(0,1fr)]">
      {#if announcements.length > 1 && ready}
        <div class="flex shrink-0 items-center gap-1" role="group" aria-label="公告播放">
          <span class="mr-2 min-w-10 text-center font-mono text-xs text-fg-muted" aria-live="off"
            >{current + 1} / {announcements.length}</span
          >
          <button
            type="button"
            class={controlClass}
            aria-label="上一条公告"
            title="上一条公告"
            onclick={() => playback?.move(-1)}
            ><IconArrowLeft size={16} aria-hidden="true" /></button
          >
          <button
            type="button"
            class={controlClass}
            aria-label={userPaused ? '播放公告' : '暂停播放'}
            title={userPaused ? '播放公告' : '暂停播放'}
            onclick={togglePlayback}
            >{#if userPaused}<IconPlayerPlay size={16} aria-hidden="true" />{:else}<IconPlayerPause
                size={16}
                aria-hidden="true"
              />{/if}</button
          >
          <button
            type="button"
            class={controlClass}
            aria-label="下一条公告"
            title="下一条公告"
            onclick={() => playback?.move(1)}
            ><IconArrowRight size={16} aria-hidden="true" /></button
          >
        </div>
      {/if}
      {#if activeAnnouncement}
        <div class="flex min-w-0 items-center justify-end gap-4 sm:col-start-2">
          {#if activeAnnouncement.action}
            <a
              class="inline-flex min-h-11 min-w-0 items-center gap-1 text-sm font-medium text-tint-fg hover:underline sm:min-h-10"
              href={activeAnnouncement.action.href}
              target={activeAnnouncement.action.external ? '_blank' : undefined}
              rel={activeAnnouncement.action.external ? 'noopener noreferrer' : undefined}
              ><span class="line-clamp-1 wrap-anywhere">{activeAnnouncement.action.label}</span
              >{#if activeAnnouncement.action.external}<IconExternalLink
                  size={16}
                  class="shrink-0"
                  aria-hidden="true"
                />{/if}</a
            >
          {/if}
          <a
            class="inline-flex min-h-11 shrink-0 items-center gap-1 text-sm font-medium text-tint-fg hover:underline sm:min-h-10"
            href={`/announcements/${activeAnnouncement.id}`}
            >查看公告<IconArrowRight size={16} aria-hidden="true" /></a
          >
        </div>
      {/if}
    </div>
  </section>
{/if}
