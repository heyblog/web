<script lang="ts">
  import { annotationValues } from '@/application/site-annotation/annotation-values';

  let { values }: { values: Readonly<Record<string, unknown>> } = $props();
  const fields = $derived(annotationValues(values));
</script>

{#if fields.length}
  <dl class="divide-y divide-line text-sm">
    {#each fields as field (field.name)}
      <div class="grid min-w-0 gap-1 py-2">
        <dt class="font-medium text-fg-muted">{field.name}</dt>
        <dd class="min-w-0">
          {#if field.entries.length}
            <ul class="grid gap-1">
              {#each field.entries as entry, index (index)}
                <li class="wrap-anywhere whitespace-pre-wrap">{entry}</li>
              {/each}
            </ul>
          {:else}<span class="text-fg-muted">无</span>{/if}
        </dd>
      </div>
    {/each}
  </dl>
{:else}
  <p class="text-sm text-fg-muted">无记录。</p>
{/if}
