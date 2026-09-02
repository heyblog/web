<script lang="ts">
  import { IconArrowsShuffle } from '@tabler/icons-svelte';

  import type {
    SiteDirectoryOrder,
    SiteDirectorySort,
    SiteDirectoryStatus,
    SiteDirectoryView,
  } from '@/application/site-directory/site-directory.models';

  type Props = {
    readonly view: SiteDirectoryView;
    readonly pending: boolean;
    readonly onStatusChange: (status: SiteDirectoryStatus) => void;
    readonly onSortChange: (sort: SiteDirectorySort) => void;
    readonly onOrderChange: (order: SiteDirectoryOrder) => void;
    readonly onShuffle: () => void;
  };

  const statuses = ['normal', 'abnormal'] as const satisfies readonly SiteDirectoryStatus[];
  let { view, pending, onStatusChange, onSortChange, onOrderChange, onShuffle }: Props = $props();

  function handleTabKeydown(event: KeyboardEvent, status: SiteDirectoryStatus): void {
    const currentIndex = statuses.indexOf(status);
    let nextIndex: number;
    switch (event.key) {
      case 'ArrowLeft':
        nextIndex = (currentIndex - 1 + statuses.length) % statuses.length;
        break;
      case 'ArrowRight':
        nextIndex = (currentIndex + 1) % statuses.length;
        break;
      case 'Home':
        nextIndex = 0;
        break;
      case 'End':
        nextIndex = statuses.length - 1;
        break;
      default:
        return;
    }
    event.preventDefault();
    const nextStatus = statuses[nextIndex];
    if (!nextStatus) return;
    const tabs =
      event.currentTarget.parentElement?.querySelectorAll<HTMLButtonElement>('[role="tab"]');
    tabs?.[nextIndex]?.focus();
    onStatusChange(nextStatus);
  }

  function parseSort(value: string): SiteDirectorySort {
    return value === 'joined' || value === 'updated' ? value : 'random';
  }

  function parseOrder(value: string): SiteDirectoryOrder {
    return value === 'asc' ? 'asc' : 'desc';
  }
</script>

<div class="border-b border-line">
  <div class="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
    <div class="flex min-h-10 items-end gap-5" role="tablist" aria-label="站点状态">
      {#each statuses as status (status)}
        {@const active = view.query.status === status}
        <button
          class={[
            'relative min-h-10 px-1 text-sm font-medium transition-colors duration-(--motion-fast)',
            active ? 'text-fg' : 'text-fg-muted hover:text-fg',
          ]}
          type="button"
          role="tab"
          aria-selected={active}
          aria-controls="site-directory-results"
          tabindex={active ? 0 : -1}
          disabled={pending}
          onclick={() => onStatusChange(status)}
          onkeydown={(event) => handleTabKeydown(event, status)}
        >
          {status === 'normal' ? '正常' : '异常'}
          <span class="ml-1 font-mono text-xs text-fg-muted">
            {view.statusCounts[status].toLocaleString('zh-CN')}
          </span>
          {#if active}
            <span class="absolute inset-x-0 bottom-0 h-0.5 bg-primary" aria-hidden="true"></span>
          {/if}
        </button>
      {/each}
    </div>

    <div class="flex flex-wrap items-center gap-2 pb-3 sm:pb-2">
      <label class="sr-only" for="directory-sort">排序方式</label>
      <select
        class="min-h-11 rounded-md border border-line-strong bg-surface px-3 text-sm font-medium transition-colors duration-(--motion-fast) outline-none focus:border-focus focus:ring-2 focus:ring-focus/30 sm:min-h-10"
        id="directory-sort"
        value={view.query.sort}
        onchange={(event) => onSortChange(parseSort(event.currentTarget.value))}
      >
        <option value="random">稳定随机</option>
        <option value="joined">加入时间</option>
        <option value="updated">资料更新时间</option>
      </select>
      {#if view.query.sort === 'random'}
        <button
          class="inline-flex min-h-11 items-center gap-1.5 rounded-md px-3 text-sm font-medium text-fg-muted transition-[background-color,color,scale] duration-(--motion-fast) hover:bg-subtle hover:text-fg active:scale-96 disabled:pointer-events-none disabled:opacity-50 sm:min-h-10"
          type="button"
          disabled={pending}
          onclick={onShuffle}
        >
          <IconArrowsShuffle aria-hidden="true" size={17} stroke={1.8} />
          换一组
        </button>
      {:else}
        <label class="sr-only" for="directory-order">排序方向</label>
        <select
          class="min-h-11 rounded-md border border-line-strong bg-surface px-3 text-sm font-medium transition-colors duration-(--motion-fast) outline-none focus:border-focus focus:ring-2 focus:ring-focus/30 sm:min-h-10"
          id="directory-order"
          value={view.query.order}
          onchange={(event) => onOrderChange(parseOrder(event.currentTarget.value))}
        >
          <option value="desc">从新到旧</option>
          <option value="asc">从旧到新</option>
        </select>
      {/if}
    </div>
  </div>
</div>
