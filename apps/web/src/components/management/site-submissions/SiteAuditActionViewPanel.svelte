<script lang="ts">
  import type {
    SiteSnapshotDraft,
    SiteSubmissionOptions,
  } from '@/application/management/site-management.snapshot';
  import FormMessage from '@/shared/ui/FormMessage.svelte';

  import SiteSnapshotEditor from '../sites/SiteSnapshotEditor.svelte';

  import type { AuditDetail } from './site-audit-review.types';

  let {
    detail,
    mode = 'detail',
    canEditSnapshot = false,
    pending = false,
    correctionDraft,
    options,
  }: {
    detail: AuditDetail;
    mode?: 'detail' | 'process';
    canEditSnapshot?: boolean;
    pending?: boolean;
    correctionDraft: SiteSnapshotDraft;
    options: SiteSubmissionOptions;
  } = $props();

  type EditableAuditField =
    | 'name'
    | 'url'
    | 'sign'
    | 'main_tag'
    | 'sub_tags'
    | 'feed'
    | 'sitemap'
    | 'link_page'
    | 'architecture';

  type FieldAlert = {
    label: string;
    value: string;
  };

  const EDITABLE_AUDIT_FIELDS = new Set<EditableAuditField>([
    'name',
    'url',
    'sign',
    'main_tag',
    'sub_tags',
    'feed',
    'sitemap',
    'link_page',
    'architecture',
  ]);

  const isInlineTagField = (field: string) => field === 'main_tag' || field === 'sub_tags';

  const buildFieldAlerts = (
    changes: NonNullable<AuditDetail['action_view']['changes']>,
  ): Partial<Record<EditableAuditField, FieldAlert>> =>
    changes.reduce<Partial<Record<EditableAuditField, FieldAlert>>>((accumulator, change) => {
      if (!EDITABLE_AUDIT_FIELDS.has(change.field as EditableAuditField)) {
        return accumulator;
      }

      accumulator[change.field as EditableAuditField] = {
        label: change.label,
        value: change.before_display || '—',
      };
      return accumulator;
    }, {});

  let fieldAlerts = $derived(
    canEditSnapshot && detail.action_view.kind === 'UPDATE'
      ? buildFieldAlerts(detail.action_view.changes ?? [])
      : {},
  );
</script>

<article class="page-section space-y-4">
  {#if mode === 'detail' && detail.action_view.kind === 'CREATE'}
    <section class="space-y-3">
      <h3 class="text-sm font-medium">提交信息</h3>
      <div class="rounded-sm border border-(--color-line)">
        <dl class="divide-y divide-(--color-line)">
          {#each detail.action_view.submitted_fields ?? [] as item (item.field)}
            <div class="grid gap-2 px-4 py-3 md:grid-cols-[11rem_minmax(0,1fr)]">
              <dt class="text-xs text-(--color-fg-3)">{item.label}</dt>
              <dd
                class={isInlineTagField(item.field)
                  ? 'overflow-x-auto text-sm whitespace-nowrap'
                  : 'text-sm break-all whitespace-pre-wrap'}
              >
                {item.value_display || '—'}
              </dd>
            </div>
          {/each}
        </dl>
      </div>
    </section>
  {/if}

  {#if mode === 'detail' && detail.action_view.kind === 'UPDATE'}
    <section class="space-y-3">
      <h3 class="text-sm font-medium">修改字段</h3>
      {#if (detail.action_view.changes?.length ?? 0) === 0}
        <p class="text-sm text-(--color-fg-3)">没有可展示的修改字段。</p>
      {/if}
      {#each detail.action_view.changes ?? [] as change (change.field)}
        <article class="rounded-sm border border-(--color-line)">
          <header class="border-b border-(--color-line) px-4 py-3 text-sm font-medium">
            {change.label}
          </header>
          <div class="space-y-2 p-3">
            <div
              class="rounded-sm border border-[color-mix(in_srgb,var(--color-fail)_30%,var(--color-line))] bg-[color-mix(in_srgb,var(--color-fail)_7%,transparent)] px-3 py-2"
            >
              <p class="text-[11px] tracking-[0.18em] text-(--color-fg-3) uppercase">修改前</p>
              {#if isInlineTagField(change.field)}
                <p class="mt-2 overflow-x-auto text-xs whitespace-nowrap">
                  {change.before_display || '—'}
                </p>
              {:else}
                <pre class="mt-2 text-xs whitespace-pre-wrap">{change.before_display || '—'}</pre>
              {/if}
            </div>
            <div
              class="rounded-sm border border-[color-mix(in_srgb,var(--color-ok)_30%,var(--color-line))] bg-[color-mix(in_srgb,var(--color-ok)_8%,transparent)] px-3 py-2"
            >
              <p class="text-[11px] tracking-[0.18em] text-(--color-fg-3) uppercase">修改后</p>
              {#if isInlineTagField(change.field)}
                <p class="mt-2 overflow-x-auto text-xs whitespace-nowrap">
                  {change.after_display || '—'}
                </p>
              {:else}
                <pre class="mt-2 text-xs whitespace-pre-wrap">{change.after_display || '—'}</pre>
              {/if}
            </div>
          </div>
        </article>
      {/each}
    </section>
  {/if}

  {#if mode === 'detail' && (detail.action_view.kind === 'DELETE' || detail.action_view.kind === 'RESTORE')}
    <section class="space-y-3">
      <h3 class="text-sm font-medium">
        {detail.action_view.kind === 'DELETE' ? '站点简要信息' : '恢复目标信息'}
      </h3>
      <div class="rounded-sm border border-(--color-line)">
        <dl class="divide-y divide-(--color-line)">
          {#each detail.action_view.site_fields ?? [] as item (item.field)}
            <div class="grid gap-2 px-4 py-3 md:grid-cols-[11rem_minmax(0,1fr)]">
              <dt class="text-xs text-(--color-fg-3)">{item.label}</dt>
              <dd
                class={isInlineTagField(item.field)
                  ? 'overflow-x-auto text-sm whitespace-nowrap'
                  : 'text-sm break-all whitespace-pre-wrap'}
              >
                {item.value_display || '—'}
              </dd>
            </div>
          {/each}
        </dl>
      </div>

      <div class="rounded-sm border border-(--color-line) px-4 py-3">
        <p class="text-xs text-(--color-fg-3)">
          {detail.action_view.kind === 'DELETE' ? '删除原因' : '恢复说明'}
        </p>
        <p class="mt-2 text-sm leading-7 whitespace-pre-wrap">
          {detail.action_view.reason ?? detail.submit_reason ?? '—'}
        </p>
      </div>
    </section>
  {/if}

  {#if mode === 'process'}
    {#if canEditSnapshot}
      <section class="space-y-3">
        <h3 class="text-sm font-medium">
          {detail.action_view.kind === 'CREATE' ? '新增信息纠正' : '修改信息纠正'}
        </h3>
        <div class="rounded-md border border-(--color-line-med) p-5 sm:px-10 sm:py-8">
          <SiteSnapshotEditor
            bind:draft={correctionDraft}
            {fieldAlerts}
            {options}
            disabled={pending}
            idPrefix="audit-correction"
            showManagement={false}
            showReadonly={false}
          />
        </div>
      </section>
    {:else}
      <FormMessage
        tone="warning"
        title="当前申请不支持信息纠正"
        message="只有新增和修改申请支持进入信息纠正页面。"
      />
    {/if}
  {/if}
</article>
