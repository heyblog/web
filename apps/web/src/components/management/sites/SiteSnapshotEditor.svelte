<script lang="ts">
  import {
    SITE_ACCESS_SCOPE_OPTIONS,
    SITE_STATUS_OPTIONS,
    type SiteSnapshotDraft,
    type SiteSubmissionOptions,
  } from '@/application/management/site-management.snapshot';
  import {
    WORKSPACE_SELECT_CHEVRON_STYLE,
    WORKSPACE_SELECT_CLASS,
  } from '@/components/site-submission/site-submission-workspace.constants';

  import ManagementReadonlyFieldsPanel from './ManagementReadonlyFieldsPanel.svelte';
  import ManagementSiteEditableFields from './ManagementSiteEditableFields.svelte';

  type ManagementSelectKey = 'access_scope' | 'status';
  type ManagementToggleKey = 'is_show' | 'recommend';

  interface ManagementSelectField {
    key: ManagementSelectKey;
    idSuffix: string;
    label: string;
    options: Array<{ value: string; label: string }>;
  }

  interface ManagementToggleField {
    key: ManagementToggleKey;
    label: string;
    description: string;
  }

  const MANAGEMENT_SELECT_FIELDS: ManagementSelectField[] = [
    {
      key: 'access_scope',
      idSuffix: 'access-scope',
      label: '访问范围',
      options: SITE_ACCESS_SCOPE_OPTIONS,
    },
    {
      key: 'status',
      idSuffix: 'site-status',
      label: '站点状态',
      options: SITE_STATUS_OPTIONS,
    },
  ];

  const MANAGEMENT_TOGGLE_FIELDS: ManagementToggleField[] = [
    {
      key: 'recommend',
      label: '推荐站点',
      description: '用于首页或目录中的推荐展示位。',
    },
    {
      key: 'is_show',
      label: '前台显示',
      description: '控制站点是否在前台目录中展示。',
    },
  ];

  let {
    draft = $bindable(),
    options,
    disabled = false,
    idPrefix = 'site-snapshot',
    showManagement = true,
    showReadonly = true,
    fieldAlerts = {},
  }: {
    draft: SiteSnapshotDraft;
    options: SiteSubmissionOptions;
    disabled?: boolean;
    idPrefix?: string;
    showManagement?: boolean;
    showReadonly?: boolean;
    fieldAlerts?: Partial<Record<string, { label: string; value: string }>>;
  } = $props();

  const updateDraft = (patch: Partial<SiteSnapshotDraft>) => {
    draft = {
      ...draft,
      ...patch,
    };
  };

  const handleManagementSelectChange = (key: ManagementSelectKey, event: Event): void => {
    const value = (event.currentTarget as HTMLSelectElement).value as SiteSnapshotDraft[typeof key];
    updateDraft({ [key]: value } as Partial<SiteSnapshotDraft>);
  };

  const handleManagementToggleChange = (key: ManagementToggleKey, event: Event): void => {
    const value = (event.currentTarget as HTMLInputElement)
      .checked as SiteSnapshotDraft[typeof key];
    updateDraft({ [key]: value } as Partial<SiteSnapshotDraft>);
  };
</script>

<div class="space-y-6">
  <section class="space-y-4">
    <ManagementSiteEditableFields bind:draft {options} {disabled} {idPrefix} {fieldAlerts} />
  </section>

  {#if showManagement}
    <section class="space-y-4 border-t border-(--color-line) pt-5">
      <div class="space-y-1">
        <p class="text-xs tracking-[0.16em] text-(--color-fg-3)">管理信息</p>
      </div>

      <div class="grid gap-4 md:grid-cols-2">
        {#each MANAGEMENT_SELECT_FIELDS as field (field.key)}
          <div class="space-y-2">
            {#if fieldAlerts[field.key]}
              <div
                class="rounded-sm border border-[color-mix(in_srgb,var(--color-fail)_32%,var(--color-line))] bg-[color-mix(in_srgb,var(--color-fail)_8%,transparent)] px-3 py-2"
              >
                <p class="text-[11px] tracking-[0.18em] text-(--color-fg-3) uppercase">修改前</p>
                <p class="mt-1 text-xs whitespace-pre-wrap text-(--color-fg)">
                  {fieldAlerts[field.key]?.value || '—'}
                </p>
              </div>
            {/if}
            <label class="block text-sm" for={`${idPrefix}-${field.idSuffix}`}>{field.label}</label>
            <select
              id={`${idPrefix}-${field.idSuffix}`}
              class={WORKSPACE_SELECT_CLASS}
              style={WORKSPACE_SELECT_CHEVRON_STYLE}
              {disabled}
              value={draft[field.key]}
              onchange={(event) => handleManagementSelectChange(field.key, event)}
            >
              {#each field.options as item (item.value)}
                <option value={item.value}>{item.label}</option>
              {/each}
            </select>
          </div>
        {/each}
      </div>

      <div class="grid gap-3 md:grid-cols-2">
        {#each MANAGEMENT_TOGGLE_FIELDS as field (field.key)}
          <div class="space-y-2">
            {#if fieldAlerts[field.key]}
              <div
                class="rounded-sm border border-[color-mix(in_srgb,var(--color-fail)_32%,var(--color-line))] bg-[color-mix(in_srgb,var(--color-fail)_8%,transparent)] px-3 py-2"
              >
                <p class="text-[11px] tracking-[0.18em] text-(--color-fg-3) uppercase">修改前</p>
                <p class="mt-1 text-xs whitespace-pre-wrap text-(--color-fg)">
                  {fieldAlerts[field.key]?.value || '—'}
                </p>
              </div>
            {/if}
            <label class="toggle-check">
              <input
                type="checkbox"
                checked={draft[field.key]}
                {disabled}
                onchange={(event) => handleManagementToggleChange(field.key, event)}
              />
              <span class="toggle-check-box" aria-hidden="true"></span>
              <span class="toggle-check-copy">
                <strong>{field.label}</strong>
                <small>{field.description}</small>
              </span>
            </label>
          </div>
        {/each}
      </div>
    </section>
  {/if}

  {#if showReadonly}
    <ManagementReadonlyFieldsPanel {draft} />
  {/if}
</div>
