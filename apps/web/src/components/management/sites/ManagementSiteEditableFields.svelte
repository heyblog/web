<script lang="ts">
  import type {
    SiteSnapshotDraft,
    SiteSubmissionOptions,
  } from '@/application/management/site-management.snapshot';

  import ManagementSiteArchitectureSection from './ManagementSiteArchitectureSection.svelte';
  import ManagementSiteBasicSection from './ManagementSiteBasicSection.svelte';
  import ManagementSiteFeedSection from './ManagementSiteFeedSection.svelte';

  let {
    draft = $bindable(),
    options,
    disabled = false,
    idPrefix = 'site-management-fields',
    fieldAlerts = {},
  }: {
    draft: SiteSnapshotDraft;
    options: SiteSubmissionOptions;
    disabled?: boolean;
    idPrefix?: string;
    fieldAlerts?: Partial<Record<string, { label: string; value: string }>>;
  } = $props();
</script>

<div class="space-y-6">
  <ManagementSiteBasicSection bind:draft {options} {disabled} {idPrefix} {fieldAlerts} />
  <ManagementSiteFeedSection bind:draft {disabled} {idPrefix} {fieldAlerts} />
  <ManagementSiteArchitectureSection bind:draft {options} {disabled} {idPrefix} {fieldAlerts} />
</div>
