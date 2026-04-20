<script lang="ts">
  import {
    FROM_SOURCE_OPTIONS,
    SITE_CLASSIFICATION_STATUS_OPTIONS,
    type SiteSnapshotDraft,
  } from '@/application/management/site-management.snapshot';

  let {
    draft,
    title = '只读字段',
    note = '以下字段由系统、审核流或自动脚本维护。',
  }: {
    draft: SiteSnapshotDraft;
    title?: string;
    note?: string;
  } = $props();

  const withOptionalClass = (baseClass: string, extraClass?: string): string =>
    extraClass ? `${baseClass} ${extraClass}` : baseClass;

  const readOptionLabel = (
    values: Array<{ value: string; label: string }>,
    value: string | null | undefined,
  ): string => values.find((item) => item.value === value)?.label ?? value?.trim() ?? '—';

  const sourceLabels = (values: string[]): string =>
    values.length > 0
      ? values.map((value) => readOptionLabel(FROM_SOURCE_OPTIONS, value)).join('; ')
      : '—';

  let readonlyFields = $derived([
    {
      label: '站点 BID',
      value: draft.bid || '—',
    },
    {
      label: '分类状态',
      value: readOptionLabel(SITE_CLASSIFICATION_STATUS_OPTIONS, draft.classification_status),
    },
    {
      label: '来源渠道',
      value: sourceLabels(draft.from),
      className: 'md:col-span-2',
    },
    {
      label: '备注原因',
      value: draft.reason || '—',
      className: 'md:col-span-2',
      valueClassName: 'whitespace-pre-wrap',
    },
  ]);
</script>

<section class="border-t border-(--color-line) pt-5">
  <details class="sidebar-collapsible management-readonly-panel">
    <summary class="sidebar-collapsible-toggle">
      <span class="management-collapsible-summary-copy">
        <span class="sidebar-collapsible-title">{title}</span>
        <span class="management-collapsible-summary-note">{note}</span>
      </span>
      <span class="sidebar-collapsible-icon" aria-hidden="true">⌄</span>
    </summary>

    <div class="sidebar-collapsible-body">
      <dl class="management-readonly-grid md:grid-cols-2">
        {#each readonlyFields as field (field.label)}
          <div class={withOptionalClass('management-readonly-item', field.className)}>
            <dt class="management-readonly-label">{field.label}</dt>
            <dd class={withOptionalClass('management-readonly-value', field.valueClassName)}>
              {field.value}
            </dd>
          </div>
        {/each}
      </dl>
    </div>
  </details>
</section>
