<script lang="ts">
  import type { BackupInspection } from '../../../api/database-backup/database-backup.types.ts';
  import {
    backupDatasetLabels,
    backupIssueMessage,
  } from '../../../application/database-backup/database-backup.messages.ts';
  let { inspection }: { inspection: BackupInspection } = $props();
  const records = $derived(
    inspection.datasets.reduce((total, dataset) => total + dataset.count, 0),
  );
</script>

<section class="mt-6 grid gap-5 border-t border-line pt-6" aria-labelledby="backup-check-heading">
  <div class="flex flex-wrap items-center justify-between gap-3">
    <h3 id="backup-check-heading" class="text-base font-semibold">文件检查</h3>
    <span
      class="rounded-full px-3 py-1 text-xs font-medium {inspection.can_restore
        ? 'bg-success-bg text-success-fg'
        : 'bg-danger-bg text-danger-fg'}"
    >
      {inspection.can_restore ? '可以恢复' : '无法恢复'}
    </span>
  </div>
  <dl class="grid grid-cols-2 gap-4 sm:grid-cols-3">
    <div>
      <dt class="text-xs text-fg-muted">数据记录</dt>
      <dd class="mt-1 font-mono text-xl font-medium">{records.toLocaleString('zh-CN')}</dd>
    </div>
    <div>
      <dt class="text-xs text-fg-muted">图节点</dt>
      <dd class="mt-1 font-mono text-xl font-medium">
        {inspection.graph.vertices.toLocaleString('zh-CN')}
      </dd>
    </div>
    <div>
      <dt class="text-xs text-fg-muted">图关系</dt>
      <dd class="mt-1 font-mono text-xl font-medium">
        {inspection.graph.edges.toLocaleString('zh-CN')}
      </dd>
    </div>
  </dl>
  <p class="text-sm text-fg-muted">
    导出时间：<time datetime={inspection.generated_at}
      >{new Date(inspection.generated_at).toLocaleString('zh-CN')}</time
    >
  </p>
  <div class="grid gap-3 rounded-md bg-subtle p-4">
    <h4 class="text-sm font-medium">管理员关联迁移</h4>
    <dl class="grid gap-3 text-sm">
      <div>
        <dt class="text-fg-muted">原管理员 ID</dt>
        <dd class="mt-1 font-mono text-xs break-all">{inspection.excluded_system_admin_id}</dd>
      </div>
      <div>
        <dt class="text-fg-muted">保留的管理员 ID</dt>
        <dd class="mt-1 font-mono text-xs break-all">{inspection.retained_system_admin_id}</dd>
      </div>
    </dl>
    <p class="text-sm text-fg-muted">历史操作将关联到当前管理员，其账号和登录资料保持不变。</p>
  </div>
  {#if !inspection.target_ready}<p class="text-sm text-danger-fg" role="alert">
      目标数据库必须只包含正式系统管理员和默认初始化数据。
    </p>{/if}
  {#if inspection.issues.length > 0}<ul class="grid gap-2 text-sm text-danger-fg" role="alert">
      {#each inspection.issues as issue, index (`${issue.code}-${index}`)}<li>
          {backupIssueMessage(issue)}
        </li>{/each}
    </ul>{/if}
  <details class="rounded-md border border-line">
    <summary class="min-h-11 cursor-pointer px-4 py-3 text-sm font-medium">数据明细</summary>
    <div class="max-h-80 overflow-auto border-t border-line">
      <table class="w-full text-sm">
        <caption class="sr-only">导出文件中的数据记录数量</caption>
        <thead class="sticky top-0 bg-surface text-xs font-medium text-fg-muted"
          ><tr
            ><th scope="col" class="px-4 py-3 text-left">数据</th><th
              scope="col"
              class="px-4 py-3 text-right">记录数</th
            ></tr
          ></thead
        >
        <tbody
          >{#each inspection.datasets as dataset (dataset.name)}<tr
              class="h-12 border-t border-line"
              ><th scope="row" class="px-4 py-3 text-left font-normal"
                >{backupDatasetLabels[dataset.name]}</th
              ><td class="px-4 py-3 text-right font-mono"
                >{dataset.count.toLocaleString('zh-CN')}</td
              ></tr
            >{/each}</tbody
        >
      </table>
    </div>
  </details>
</section>
