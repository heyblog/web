<script lang="ts">
  import type { SiteCardView } from '@/application/home/home.shared';

  let { site }: { site: SiteCardView } = $props();
  const accessLabels = {
    ALL: '全球可访问',
    CN_ONLY: '仅中国大陆可访问',
    GLOBAL_ONLY: '仅海外可访问',
  } as const;
</script>

<article aria-label="随机博客预览" class="min-w-0">
  <h2 class="text-2xl/tight font-bold wrap-break-word sm:text-3xl/tight">{site.name}</h2>
  <p class="mt-3 font-mono text-sm/6 break-all text-fg-muted">{site.homepageUrl}</p>
  <p class="mt-5 text-base/7 wrap-break-word text-fg-muted">
    {site.summary.trim() || '该博客暂无简介。'}
  </p>
  <dl class="mt-6 grid gap-4 border-t border-line pt-5 text-sm sm:grid-cols-2">
    <div>
      <dt class="text-fg-muted">分类</dt>
      <dd class="mt-1 font-medium">
        {site.classification
          ? `${site.classification.level1.name} · ${site.classification.level2.name}`
          : '未分类'}
      </dd>
    </div>
    <div>
      <dt class="text-fg-muted">访问范围</dt>
      <dd class="mt-1 font-medium">{accessLabels[site.accessScope]}</dd>
    </div>
  </dl>
  {#if site.warnings.length > 0}
    <div class="mt-5 border-t border-line pt-5">
      <h3 class="text-sm font-medium">访问提示</h3>
      <ul class="mt-2 flex flex-wrap gap-2">
        {#each site.warnings as warning (warning.slug)}
          <li class="rounded-sm bg-warning-bg px-2 py-1 text-xs font-medium text-warning-fg">
            {warning.name}
          </li>
        {/each}
      </ul>
    </div>
  {/if}
</article>
