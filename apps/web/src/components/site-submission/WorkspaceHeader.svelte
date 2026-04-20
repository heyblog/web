<script lang="ts">
  import type { SubmissionPage } from '@/application/site-submission/site-submission.service';

  let {
    activePage = 'create',
  }: {
    activePage?: SubmissionPage;
  } = $props();
</script>

<div class="flex flex-wrap items-start justify-between gap-4 border-b border-(--color-line) pb-5">
  <div class="space-y-2">
    <p class="font-mono text-[11px] tracking-[0.18em] text-red-700 uppercase dark:text-red-400">
      公开提交流程
    </p>
    <h2 class="text-[clamp(1.8rem,4vw,2.7rem)] leading-[1.04] tracking-[-0.04em]">
      {#if activePage === 'create'}
        新增站点
      {:else if activePage === 'update'}
        修订站点资料
      {:else if activePage === 'delete'}
        提交删除申请
      {:else if activePage === 'restore'}
        恢复已下线站点
      {:else}
        查询审核结果
      {/if}
    </h2>
  </div>

  <nav class="flex flex-wrap gap-2 text-sm">
    <a
      class={`rounded-[4px] border px-3 py-2 transition ${
        activePage === 'create'
          ? 'border-(--color-line-med) text-(--color-fg)'
          : 'border-(--color-line) text-(--color-fg-2) hover:text-(--color-fg)'
      }`}
      href="/site/submit/"
    >
      新增
    </a>
    <a
      class={`rounded-[4px] border px-3 py-2 transition ${
        activePage === 'update'
          ? 'border-(--color-line-med) text-(--color-fg)'
          : 'border-(--color-line) text-(--color-fg-2) hover:text-(--color-fg)'
      }`}
      href="/site/submit/update"
    >
      修订
    </a>
    <a
      class={`rounded-[4px] border px-3 py-2 transition ${
        activePage === 'delete'
          ? 'border-(--color-line-med) text-(--color-fg)'
          : 'border-(--color-line) text-(--color-fg-2) hover:text-(--color-fg)'
      }`}
      href="/site/submit/delete"
    >
      删除
    </a>
    <a
      class={`rounded-[4px] border px-3 py-2 transition ${
        activePage === 'restore'
          ? 'border-(--color-line-med) text-(--color-fg)'
          : 'border-(--color-line) text-(--color-fg-2) hover:text-(--color-fg)'
      }`}
      href="/site/submit/restore"
    >
      恢复
    </a>
    <a
      class={`rounded-[4px] border px-3 py-2 transition ${
        activePage === 'query'
          ? 'border-(--color-line-med) text-(--color-fg)'
          : 'border-(--color-line) text-(--color-fg-2) hover:text-(--color-fg)'
      }`}
      href="/site/submit/query"
    >
      查询
    </a>
  </nav>
</div>
