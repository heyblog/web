<script lang="ts">
  import {
    isActiveJob,
    type RestoreJob,
  } from '../../../api/database-backup/database-backup.jobs.ts';
  import { backupDatasetLabels } from '../../../application/database-backup/database-backup.messages.ts';
  let { job }: { job: RestoreJob } = $props();
  const stages = {
    validating: '正在校验文件',
    restoring: '正在恢复数据',
    checking: '正在核对恢复数据',
    committing: '正在完成恢复',
    rolling_back: '正在回滚',
  } as const;
  const datasetLabels: Readonly<Record<string, string>> = {
    ...backupDatasetLabels,
    'graph.vertices': '关系节点',
    'graph.edges': '站点关系',
  };
  const label = $derived(job.status === 'cancelling' ? '正在回滚' : stages[job.progress.stage]);
</script>

{#if isActiveJob(job)}
  <div class="mt-5 grid gap-3 rounded-md border border-line p-4">
    <p class="text-sm font-medium" role="status">{label}</p>
    {#if job.progress.stage === 'restoring'}
      <div
        class="h-1.5 overflow-hidden rounded-full bg-subtle"
        role="progressbar"
        aria-label="数据恢复进度"
        aria-valuemin="0"
        aria-valuemax={job.progress.total}
        aria-valuenow={job.progress.completed}
      >
        <div
          class="h-full origin-left bg-primary"
          style:transform="scaleX({job.progress.total
            ? job.progress.completed / job.progress.total
            : 0})"
        ></div>
      </div>
      <p class="text-sm text-fg-muted">
        {datasetLabels[job.progress.dataset] ?? '业务数据'} · {job.progress.completed.toLocaleString()}
        / {job.progress.total.toLocaleString()} 条
      </p>
    {/if}
    <p class="text-xs text-fg-muted">可以离开此页面，返回后可继续查看进度。</p>
  </div>
{:else if job.status === 'cancelled'}
  <p class="mt-5 rounded-md bg-subtle p-4 text-sm" role="status">恢复已取消，原有数据已保留。</p>
{/if}
