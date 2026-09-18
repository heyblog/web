<script lang="ts">
  import {
    IconActivityHeartbeat,
    IconArticle,
    IconTimeline,
    IconTopologyStar3,
  } from '@tabler/icons-svelte';

  import { nextTabIndex } from '@/shared/tab-navigation';

  const tabs = [
    { id: 'articles', label: '文章信息', empty: '暂无文章信息', icon: IconArticle },
    { id: 'links', label: '友链图谱', empty: '暂无友链图谱', icon: IconTopologyStar3 },
    { id: 'logs', label: '信息日志', empty: '暂无信息日志', icon: IconTimeline },
    { id: 'checks', label: '检测记录', empty: '暂无检测记录', icon: IconActivityHeartbeat },
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
      target.parentElement?.querySelectorAll<HTMLButtonElement>('[role="tab"]')[next]?.focus();
    }
  }
</script>

<section class="min-w-0" aria-label="博客详细信息">
  <div
    class="relative grid grid-cols-4 border-b border-line"
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
        'pointer-events-none absolute bottom-0 left-0 h-0.5 w-1/4 bg-primary ease-standard motion-reduce:transition-none',
        keyboard ? 'transition-none' : 'transition-transform duration-(--motion-base)',
      ]}
      style:transform={`translateX(${selected * 100}%)`}
      aria-hidden="true"
    ></span>
  </div>
  {#each tabs as tab, index (tab.id)}
    <div
      id={`site-panel-${tab.id}`}
      role="tabpanel"
      aria-labelledby={`site-tab-${tab.id}`}
      hidden={selected !== index}
      tabindex="0"
      class="rounded-sm py-12 sm:py-16"
    >
      <div class="flex flex-col items-center gap-4 text-center">
        <span
          class="inline-flex size-12 items-center justify-center rounded-md bg-subtle text-fg-muted"
        >
          <tab.icon size={22} stroke={1.8} aria-hidden="true" />
        </span>
        <p class="text-sm font-semibold text-fg-muted">{tab.empty}</p>
      </div>
    </div>
  {/each}
</section>
