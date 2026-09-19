<script lang="ts">
  import {
    IconActivityHeartbeat,
    IconArticle,
    IconTimeline,
    IconTopologyStar3,
  } from '@tabler/icons-svelte';
  import type { Snippet } from 'svelte';

  import { nextTabIndex } from '@/shared/tab-navigation';

  interface Props {
    readonly children: Snippet;
  }
  let { children }: Props = $props();

  const tabs = [
    { id: 'basic', label: '基础信息' },
    { id: 'articles', label: '文章信息', empty: '暂无文章信息', icon: IconArticle },
    { id: 'links', label: '友链图谱', empty: '暂无友链图谱', icon: IconTopologyStar3 },
    { id: 'logs', label: '信息日志', empty: '暂无信息日志', icon: IconTimeline },
    { id: 'checks', label: '检测记录', empty: '暂无检测记录', icon: IconActivityHeartbeat },
  ] as const;
  const indicatorClasses = [
    'translate-x-0',
    'translate-x-full',
    'translate-x-[200%]',
    'translate-x-[300%]',
    'translate-x-[400%]',
  ] as const;
  let selected = $state(0);
  let keyboard = $state(false);

  function handleKeydown(event: KeyboardEvent, index: number): void {
    const next = nextTabIndex(event.key, index, tabs.length);
    if (next === undefined) return;
    event.preventDefault();
    keyboard = true;
    selected = next;
    const target = event.currentTarget;
    if (target instanceof HTMLElement) {
      const nextTab =
        target.parentElement?.querySelectorAll<HTMLButtonElement>('[role="tab"]')[next];
      nextTab?.focus({ preventScroll: true });
      nextTab?.scrollIntoView({ block: 'nearest', inline: 'nearest', behavior: 'instant' });
    }
  }
</script>

<section class="min-w-0" aria-label="博客详细信息">
  <div class="-m-1 overflow-x-auto p-1">
    <div
      class="relative grid min-w-100 grid-cols-5 border-b border-line"
      role="tablist"
      aria-label="博客信息分类"
    >
      {#each tabs as tab, index (tab.id)}
        <button
          class={[
            'min-h-11 min-w-0 px-1 text-sm font-medium whitespace-nowrap transition-colors duration-(--motion-color) hover:text-fg sm:min-h-10 sm:px-4',
            selected === index ? 'text-fg' : 'text-fg-muted',
          ]}
          id={`site-tab-${tab.id}`}
          type="button"
          role="tab"
          aria-selected={selected === index}
          aria-controls={`site-panel-${tab.id}`}
          tabindex={selected === index ? 0 : -1}
          onclick={() => {
            keyboard = false;
            selected = index;
          }}
          onkeydown={(event) => handleKeydown(event, index)}>{tab.label}</button
        >
      {/each}
      <span
        class={[
          'pointer-events-none absolute bottom-0 left-0 h-0.5 w-1/5 bg-primary ease-standard motion-reduce:transition-none',
          indicatorClasses[selected],
          keyboard ? 'transition-none' : 'transition-transform duration-(--motion-base)',
        ]}
        aria-hidden="true"
      ></span>
    </div>
  </div>
  {#each tabs as tab, index (tab.id)}
    <div
      id={`site-panel-${tab.id}`}
      role="tabpanel"
      aria-labelledby={`site-tab-${tab.id}`}
      hidden={selected !== index}
      tabindex="0"
      class={['min-w-0 rounded-sm', tab.id === 'basic' ? 'py-8' : 'py-12 sm:py-16']}
    >
      {#if tab.id === 'basic'}
        {@render children()}
      {:else}
        <div class="flex flex-col items-center gap-4 text-center">
          <span
            class="inline-flex size-12 items-center justify-center rounded-md bg-subtle text-fg-muted"
          >
            <tab.icon size={22} stroke={1.8} aria-hidden="true" />
          </span>
          <p class="text-sm font-semibold text-fg-muted">{tab.empty}</p>
        </div>
      {/if}
    </div>
  {/each}
</section>
