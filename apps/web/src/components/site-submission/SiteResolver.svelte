<script lang="ts">
  import { IconSearch } from '@tabler/icons-svelte';
  import { onDestroy, untrack } from 'svelte';

  import type { SiteSearchResult } from '@/application/site-submission/site-submission.types';
  import { isSiteShortID } from '@/application/site-submission/site-submission.validation';
  interface Props {
    initialQuery: string;
    resolving: boolean;
    onresolve: (siteShortID: string) => Promise<void>;
  }
  let { initialQuery, resolving, onresolve }: Props = $props();
  let query = $state(untrack(() => initialQuery));
  let results = $state.raw<readonly SiteSearchResult[]>([]);
  let searching = $state(false);
  let controller: AbortController | null = null;
  let searchTimer: ReturnType<typeof setTimeout> | null = null;
  onDestroy(() => {
    controller?.abort();
    if (searchTimer) clearTimeout(searchTimer);
  });
  function scheduleSearch(): void {
    if (searchTimer) clearTimeout(searchTimer);
    searchTimer = setTimeout(search, 250);
  }
  async function search(): Promise<void> {
    const value = query.trim();
    if (!value) {
      results = [];
      return;
    }
    controller?.abort();
    controller = new AbortController();
    searching = true;
    try {
      const response = await fetch(`/api/site-submissions/sites?q=${encodeURIComponent(value)}`, {
        signal: controller.signal,
      });
      if (response.ok) {
        const payload = (await response.json()) as { readonly items: readonly SiteSearchResult[] };
        results = payload.items;
      }
    } catch (caught) {
      if (!(caught instanceof DOMException && caught.name === 'AbortError')) throw caught;
    } finally {
      searching = false;
    }
  }
</script>

<section class="grid gap-3">
  <div>
    <h2 class="text-xl font-semibold">目标站点</h2>
    <p class="mt-1 text-sm text-fg-muted">
      输入 9 位 short_id 可直接读取；输入站点名称或域名后请从结果中选择。
    </p>
  </div>
  <div class="flex gap-2">
    <div class="relative flex-1">
      <IconSearch
        class="pointer-events-none absolute top-3.5 left-3 text-fg-muted"
        size={18}
        aria-hidden="true"
      /><input
        aria-label="站点名称、域名或 short_id"
        class="min-h-11 w-full rounded-sm border border-line-strong bg-surface pr-3 pl-10"
        bind:value={query}
        placeholder="站点名称、域名或 9 位 short_id"
        oninput={scheduleSearch}
      />
    </div>
    <button
      class="min-h-11 rounded-sm border border-line-strong px-4 font-medium transition-[border-color,background-color,color,scale] duration-(--motion-fast) ease-standard hover:border-primary hover:bg-tint hover:text-tint-fg active:scale-96 disabled:pointer-events-none disabled:opacity-50 motion-reduce:transition-colors"
      type="button"
      disabled={resolving || !isSiteShortID(query.trim())}
      onclick={() => onresolve(query.trim())}>{resolving ? '读取中…' : '读取'}</button
    >
  </div>
  {#if searching}<p class="text-sm text-fg-muted" role="status">正在搜索…</p>{/if}
  {#if results.length}<ul class="grid rounded-md border border-line p-1">
      {#each results as result (result.short_id)}<li>
          <button
            class="grid min-h-14 w-full rounded-sm px-3 py-2 text-left hover:bg-subtle"
            type="button"
            onclick={() => {
              query = result.short_id;
              return onresolve(result.short_id);
            }}
            ><span class="text-sm font-medium">{result.name}</span><span
              class="text-xs text-fg-muted">{result.url}</span
            ><code class="text-xs text-fg-muted">{result.short_id}</code></button
          >
        </li>{/each}
    </ul>{/if}
</section>
