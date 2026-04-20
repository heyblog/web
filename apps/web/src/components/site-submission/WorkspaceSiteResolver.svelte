<script lang="ts">
  import type {
    SiteResolveRequest,
    SiteResolveResult,
    SiteSearchItem,
  } from '@/application/site-submission/site-submission.service';

  let {
    inputClass = '',
    searchQuery = $bindable(''),
    searchPending = false,
    resolvePending = false,
    searchError = null,
    searchResults = [],
    selectedSite = null,
    runSearch,
    resolveSite,
  }: {
    inputClass?: string;
    searchQuery?: string;
    searchPending?: boolean;
    resolvePending?: boolean;
    searchError?: string | null;
    searchResults?: SiteSearchItem[];
    selectedSite?: SiteResolveResult | null;
    runSearch: () => Promise<void>;
    resolveSite: (identifier: string | SiteResolveRequest) => Promise<void>;
  } = $props();
</script>

<section class="space-y-4 rounded-md border border-(--color-line) p-4">
  <div class="flex flex-col gap-3 md:flex-row">
    <input
      class={`${inputClass} flex-1`}
      bind:value={searchQuery}
      placeholder="输入站点名称片段，或直接填写站点 ID、完整 URL、bid"
    />
    <button
      class="inline-flex min-h-11 items-center justify-center rounded-md border border-(--color-line-med) px-4 text-sm font-medium"
      type="button"
      onclick={runSearch}
      disabled={searchPending}
    >
      {searchPending ? '搜索中...' : '搜索站点'}
    </button>
    <button
      class="inline-flex min-h-11 items-center justify-center rounded-md border border-(--color-line-med) px-4 text-sm"
      type="button"
      onclick={() => resolveSite(searchQuery)}
      disabled={resolvePending}
    >
      {resolvePending ? '载入中...' : '直接载入'}
    </button>
  </div>
  {#if searchError}<p class="text-sm text-(--color-fail)">{searchError}</p>{/if}

  {#if searchResults.length > 0}
    <div class="grid gap-3">
      {#each searchResults as item (item.site_id)}
        <button
          class="rounded-md border border-(--color-line) px-4 py-3 text-left transition hover:border-red-700/25 hover:bg-(--color-bg-raised) dark:hover:border-red-400/25"
          type="button"
          onclick={() => resolveSite({ site_id: item.site_id })}
        >
          <p class="text-sm font-medium">{item.name}</p>
          <p class="mt-1 text-xs text-(--color-fg-3)">{item.url}</p>
          {#if item.bid}<p
              class="mt-1 font-mono text-[11px] tracking-[0.18em] text-(--color-fg-3) uppercase"
            >
              {item.bid}
            </p>{/if}
        </button>
      {/each}
    </div>
  {/if}
</section>

{#if selectedSite}
  <section class="rounded-md border border-(--color-line-med) p-4">
    <p class="font-mono text-[11px] tracking-[0.18em] text-(--color-fg-3) uppercase">已选择站点</p>
    <h3 class="mt-3 text-lg leading-tight">{selectedSite.name}</h3>
    <p class="mt-2 text-sm leading-7 text-(--color-fg-2)">{selectedSite.url}</p>
  </section>
{/if}
