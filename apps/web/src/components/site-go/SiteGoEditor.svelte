<script lang="ts">
  import { onMount } from 'svelte';

  import type { SiteCardView } from '@/application/home/home.shared';
  import type { SiteDirectoryClassificationOption } from '@/application/site-directory/site-directory.models';
  import {
    copySiteGoLink,
    previewRandomSite,
    type SiteGoSelection,
  } from '@/application/site-go/site-go.browser';
  import { buildSiteGoLink } from '@/application/site-go/site-go.shared';

  import SiteGoPreview from './SiteGoPreview.svelte';

  interface Props {
    classifications: readonly SiteDirectoryClassificationOption[];
    initialLevel1: string;
    initialLevel2: string;
    initialSite: SiteCardView | null;
    baseURL: string;
  }
  let { classifications, initialLevel1, initialLevel2, initialSite, baseURL }: Props = $props();
  const initial = () => ({ level1: initialLevel1, level2: initialLevel2, site: initialSite });
  let level1 = $state(initial().level1);
  let level2 = $state(initial().level2);
  let selection = $state<SiteGoSelection | null>({ kind: 'success', site: initial().site });
  let pending = $state(false);
  let ready = $state(false);
  let copyMessage = $state('');
  let copying = $state(false);
  let linkField: HTMLInputElement | undefined = $state();
  let controller: AbortController | undefined;
  const children = $derived(classifications.find((item) => item.label === level1)?.children ?? []);
  const link = $derived(buildSiteGoLink(baseURL, level1, level2));

  onMount(() => {
    ready = true;
    return () => controller?.abort();
  });

  function clearPreview(): void {
    controller?.abort();
    selection = null;
    pending = false;
    copyMessage = '';
  }

  async function preview(): Promise<void> {
    controller?.abort();
    const request = new AbortController();
    controller = request;
    pending = true;
    selection = null;
    const result = await previewRandomSite(level1, level2, request.signal);
    if (request.signal.aborted) return;
    selection = result;
    pending = false;
  }

  async function copy(): Promise<void> {
    const copiedLink = link;
    copying = true;
    const copied = await copySiteGoLink(copiedLink, navigator.clipboard);
    copying = false;
    if (link !== copiedLink) return;
    copyMessage = copied ? '链接已复制' : '复制失败，请选中链接后手动复制。';
    if (!copied) {
      linkField?.focus();
      linkField?.select();
    }
  }
</script>

<div class="grid gap-6">
  <section
    class="min-w-0 rounded-md border border-line bg-surface p-5 shadow-2xs sm:p-6"
    aria-labelledby="preview-title"
    aria-busy={pending}
  >
    <h2 id="preview-title" class="mb-5 text-sm font-medium text-fg-muted">
      博客预览 · 不会自动跳转
    </h2>
    {#if selection?.kind === 'success' && selection.site}
      <SiteGoPreview site={selection.site} />
    {:else}
      <p class="text-sm/7 text-fg-muted" role="status">
        {pending
          ? '正在随机选择博客…'
          : selection?.kind === 'error'
            ? selection.message
            : selection?.kind === 'success'
              ? '当前分类下还没有公开博客，请试试其他分类。'
              : '分类已更新，点击“预览博客”查看新的随机结果。'}
      </p>
    {/if}
    <p class="mt-5 border-t border-line pt-4 text-xs/6 text-fg-muted">
      每次打开跳转链接都会重新随机，结果可能与此处不同。
    </p>
  </section>
  <section
    class="min-w-0 rounded-md border border-line bg-surface p-5 shadow-2xs sm:p-6"
    aria-labelledby="link-editor-title"
  >
    <h2 id="link-editor-title" class="text-lg font-semibold">生成跳转链接</h2>
    <p class="mt-2 text-sm/6 text-fg-muted">
      选择分类后复制链接。访客打开链接时，会随机选中一个博客并在 10 秒后前往。
    </p>
    <label for="site-go-link" class="mt-5 block text-sm font-medium">跳转链接</label>
    <input
      bind:this={linkField}
      id="site-go-link"
      class="mt-2 min-h-11 w-full min-w-0 rounded-md border border-line-strong bg-canvas px-3 font-mono text-sm"
      readonly
      value={link}
      onfocus={(event) => event.currentTarget.select()}
    />
    <div class="mt-5 grid gap-4 border-t border-line pt-5 sm:grid-cols-2">
      <label class="grid gap-1.5 text-sm font-medium" for="site-go-level1"
        >一级分类
        <select
          id="site-go-level1"
          class="min-h-11 min-w-0 rounded-md border border-line-strong bg-canvas px-3 text-base disabled:text-fg-muted sm:min-h-10 sm:text-sm"
          value={level1}
          disabled={!ready}
          onchange={(event) => {
            level1 = event.currentTarget.value;
            level2 = '';
            clearPreview();
          }}
        >
          <option value="">全部分类</option>
          {#each classifications as option (option.value)}<option value={option.label}
              >{option.label}</option
            >{/each}
        </select>
      </label>
      <label class="grid gap-1.5 text-sm font-medium" for="site-go-level2"
        >二级分类
        <select
          id="site-go-level2"
          class="min-h-11 min-w-0 rounded-md border border-line-strong bg-canvas px-3 text-base disabled:bg-subtle disabled:text-fg-muted sm:min-h-10 sm:text-sm"
          value={level2}
          disabled={!ready || !level1}
          onchange={(event) => {
            level2 = event.currentTarget.value;
            clearPreview();
          }}
        >
          <option value="">全部分类</option>
          {#each children as option (option.value)}<option value={option.label}
              >{option.label}</option
            >{/each}
        </select>
      </label>
    </div>
    <div class="mt-4 flex flex-wrap gap-2">
      <button
        class="inline-flex min-h-11 items-center justify-center rounded-md bg-primary px-4 text-sm font-semibold text-primary-fg transition-colors duration-(--motion-fast) hover:bg-primary-hover disabled:opacity-50 sm:min-h-10"
        type="button"
        disabled={!ready || copying}
        onclick={copy}>{copying ? '正在复制…' : '复制链接'}</button
      >
      <button
        class="inline-flex min-h-11 items-center justify-center rounded-md border border-line-strong px-4 text-sm font-medium transition-colors duration-(--motion-fast) hover:bg-subtle disabled:opacity-50 sm:min-h-10"
        type="button"
        disabled={!ready || pending}
        onclick={preview}>{pending ? '正在预览…' : '预览博客'}</button
      >
    </div>
    <p class="mt-3 text-sm/6 text-fg-muted" role="status">{copyMessage}</p>
    <noscript
      ><p class="mt-3 text-sm/6 text-fg-muted">
        启用 JavaScript 后可编辑分类和预览博客；当前链接可直接选中复制。
      </p></noscript
    >
  </section>
</div>
