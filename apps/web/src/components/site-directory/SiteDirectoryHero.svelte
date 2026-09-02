<script lang="ts">
  import { IconSearch } from '@tabler/icons-svelte';

  type Props = {
    readonly searchDraft: string;
    readonly pending: boolean;
    readonly onSearchInput: (value: string) => void;
    readonly onSearchSubmit: () => void;
  };

  let { searchDraft, pending, onSearchInput, onSearchSubmit }: Props = $props();
</script>

<section class="border-b border-line bg-surface">
  <div
    class="mx-auto w-[min(calc(100%-2rem),80rem)] py-8 sm:w-[min(calc(100%-3rem),80rem)] sm:py-10"
  >
    <p class="font-mono text-xs font-medium text-tint-fg">独立博客目录</p>
    <div class="mt-2">
      <div>
        <h1 class="text-3xl/tight font-bold sm:text-4xl/tight">博客列表</h1>
        <p class="mt-3 max-w-2xl text-base/7 text-fg-muted">
          按主标签、子标签、技术与访问范围浏览 HeyBlog 收录的独立博客。
        </p>
      </div>
    </div>

    <form
      class="mt-6 flex flex-col gap-2 sm:flex-row"
      onsubmit={(event) => {
        event.preventDefault();
        onSearchSubmit();
      }}
    >
      <label class="relative min-w-0 flex-1">
        <span class="sr-only">搜索博客名称、域名或简介</span>
        <IconSearch
          class="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-fg-muted"
          aria-hidden="true"
          size={18}
          stroke={1.8}
        />
        <input
          class="min-h-11 w-full rounded-md border border-line-strong bg-canvas pr-3 pl-10 text-sm transition-colors duration-(--motion-fast) outline-none placeholder:text-fg-muted focus:border-focus focus:ring-2 focus:ring-focus/30 sm:min-h-10"
          type="search"
          value={searchDraft}
          maxlength="100"
          placeholder="搜索博客名称、域名或简介"
          oninput={(event) => onSearchInput(event.currentTarget.value)}
        />
      </label>
      <button
        class="inline-flex min-h-11 items-center justify-center rounded-md bg-primary-hover px-5 text-sm font-semibold text-white transition-[background-color,scale] duration-(--motion-fast) hover:bg-primary-active active:scale-96 disabled:pointer-events-none disabled:opacity-50 sm:min-h-10"
        type="submit"
        disabled={pending}
      >
        搜索
      </button>
    </form>
  </div>
</section>
