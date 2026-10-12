<script lang="ts">
  import { IconSearch } from '@tabler/icons-svelte';
  import { onDestroy, untrack } from 'svelte';

  import { createSiteSearch } from '@/application/site-submission/site-search.controller.svelte';
  import { isSiteShortID } from '@/application/site-submission/site-submission.validation';
  interface Props {
    initialQuery: string;
    resolving: boolean;
    onresolve: (siteShortID: string) => Promise<void>;
    onclear?: () => void;
  }
  let { initialQuery, resolving, onresolve, onclear }: Props = $props();
  let query = $state(untrack(() => initialQuery));
  const search = createSiteSearch();
  onDestroy(() => search.dispose());
  function scheduleSearch() {
    onclear?.();
    search.schedule(query);
  }
</script>

<section class="grid gap-3">
  <div>
    <h2 class="text-xl font-semibold">目标站点</h2>
    <p class="mt-1 text-sm text-fg-muted">输入站点名称、域名或标识符后选择站点。</p>
  </div>
  <div class="grid gap-2 xs:grid-cols-[minmax(0,1fr)_auto]">
    <div class="relative min-w-0">
      <IconSearch
        class="pointer-events-none absolute top-3.5 left-3 text-fg-muted"
        size={18}
        aria-hidden="true"
      /><input
        aria-label="站点名称、域名或标识符"
        class="min-h-11 w-full rounded-sm border border-line-strong bg-surface pr-3 pl-10"
        bind:value={query}
        placeholder="站点名称、域名或标识符"
        disabled={resolving}
        oninput={scheduleSearch}
      />
    </div>
    <button
      class="min-h-11 w-full rounded-sm border border-line-strong px-4 font-medium transition-[border-color,background-color,color,scale] duration-(--motion-fast) ease-standard hover:border-primary hover:bg-tint hover:text-tint-fg active:scale-96 disabled:pointer-events-none disabled:opacity-50 motion-reduce:transition-colors xs:w-auto"
      type="button"
      disabled={resolving || !isSiteShortID(query.trim())}
      onclick={() => {
        search.clear();
        return onresolve(query.trim());
      }}>{resolving ? '读取中…' : '读取'}</button
    >
  </div>
  {#if search.error}<p class="text-sm text-danger-fg" role="alert">{search.error}</p>
    <button
      type="button"
      class="min-h-11 w-fit text-sm text-tint-fg underline"
      onclick={() => void search.search(query)}>重新搜索</button
    >{:else if search.searched && search.results.length === 0}<p
      class="text-sm text-fg-muted"
      role="status"
    >
      未找到匹配的站点。
    </p>{/if}
  {#if search.searching}<p class="text-sm text-fg-muted" role="status">正在搜索…</p>{/if}
  {#if search.results.length}<ul class="grid rounded-md border border-line p-1">
      {#each search.results as result (result.short_id)}<li>
          <button
            class="grid min-h-14 w-full rounded-sm px-3 py-2 text-left hover:bg-subtle"
            type="button"
            disabled={resolving}
            onclick={() => {
              search.clear();
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
