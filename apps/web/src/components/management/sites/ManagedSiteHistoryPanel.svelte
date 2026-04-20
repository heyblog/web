<script lang="ts">
  import type { SiteHistoryItem } from './managed-site-history.shared';

  let {
    history = [],
  }: {
    history?: SiteHistoryItem[];
  } = $props();

  const actionLabelMap: Record<string, string> = {
    CREATE: '新增',
    UPDATE: '修改',
    DELETE: '删除',
  };
</script>

<section class="page-section space-y-4 pt-2">
  <div class="section-head">
    <div>
      <p class="eyebrow">
        <span class="status-dot" style="--status-dot: var(--color-info-dot)"></span>
        sites / history
      </p>
      <h2 class="section-title">最近修改记录</h2>
    </div>
  </div>

  <div class="overflow-x-auto rounded-sm border border-(--color-line)">
    <table class="min-w-full border-collapse text-left text-sm">
      <thead class="border-b border-(--color-line) bg-(--color-bg-raised)">
        <tr>
          <th class="px-4 py-3 align-middle font-medium">时间</th>
          <th class="px-4 py-3 align-middle font-medium">动作</th>
          <th class="px-4 py-3 align-middle font-medium">操作者</th>
          <th class="px-4 py-3 align-middle font-medium">关键摘要</th>
          <th class="px-4 py-3 align-middle font-medium">备注</th>
        </tr>
      </thead>
      <tbody>
        {#if history.length === 0}
          <tr>
            <td class="px-4 py-10 text-center text-(--color-fg-3)" colspan="5"> 暂无历史记录。 </td>
          </tr>
        {/if}
        {#each history as item (item.id)}
          <tr class="border-b border-(--color-line) last:border-b-0">
            <td class="px-4 py-4 align-middle text-xs text-(--color-fg-3)">{item.created_time}</td>
            <td class="px-4 py-4 align-middle">{actionLabelMap[item.action] ?? item.action}</td>
            <td class="px-4 py-4 align-middle">{item.operator_name}</td>
            <td class="px-4 py-4 align-middle text-xs">{item.change_summary}</td>
            <td class="px-4 py-4 align-middle text-xs text-(--color-fg-3)">
              {item.reviewer_comment ?? item.submit_reason ?? '—'}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</section>
