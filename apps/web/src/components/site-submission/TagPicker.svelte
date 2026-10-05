<script lang="ts">
  import { onDestroy } from 'svelte';
  import { SvelteSet } from 'svelte/reactivity';

  import type { Option } from '@/api/site-submission/site-submission.types';
  import type { EditableSubmission } from '@/application/site-submission/site-submission.browser';
  import SlugField from '@/components/management/tags/SlugField.svelte';

  import ClassificationPicker from './ClassificationPicker.svelte';
  import TertiaryTagPicker from './TertiaryTagPicker.svelte';

  interface Props {
    form: EditableSubmission;
    options: readonly Option[];
    canManageTaxonomy?: boolean;
    disabled?: boolean;
    onbusy?: (busy: boolean) => void;
  }

  let {
    form = $bindable(),
    options,
    canManageTaxonomy = false,
    disabled = false,
    onbusy,
  }: Props = $props();
  const busyTags = new SvelteSet<string>();
  let generating = $state(false);
  let disposed = false;

  function generationBusy(id: string, busy: boolean): void {
    if (disposed) return;
    if (busy) busyTags.add(id);
    else busyTags.delete(id);
    generating = busyTags.size > 0;
    onbusy?.(generating);
  }

  onDestroy(() => {
    disposed = true;
    if (busyTags.size) onbusy?.(false);
  });
  const customTags = $derived(form.tags.filter((tag) => tag.suggestedName));
</script>

<section class="grid min-w-0 gap-5">
  <div>
    <h2 class="text-xl font-semibold">标签</h2>
    <p class="mt-1 text-sm text-pretty text-fg-muted">选择固定分类，并按需添加三级标签。</p>
  </div>

  <fieldset class="grid min-w-0 gap-5" disabled={disabled || generating}>
    <ClassificationPicker bind:form {options} />

    <div class="border-t border-line pt-5">
      <TertiaryTagPicker bind:form {options} />
    </div>
  </fieldset>

  {#if canManageTaxonomy && customTags.length > 0}
    <div class="grid gap-4 border-t border-line pt-5">
      <p class="text-sm text-fg-muted">新标签将在审核保存时自动生成 slug，也可在此填写。</p>
      {#each customTags as tag (tag.id)}
        <fieldset
          class="grid min-w-0 gap-3 rounded-sm border border-line p-3"
          disabled={disabled || generating}
        >
          <legend class="max-w-full px-1 text-sm font-semibold wrap-anywhere">{tag.name}</legend>
          <SlugField
            required={false}
            disabled={disabled || generating}
            onbusy={(busy) => generationBusy(tag.id, busy)}
            bind:value={tag.slug}
            name={tag.name}
            description={tag.description}
          />
          <label class="grid min-w-0 gap-1.5 text-sm">
            标签说明（选填）
            <textarea
              class="min-h-24 min-w-0 rounded-sm border border-line-strong bg-surface p-3"
              bind:value={tag.description}
              maxlength="1000"></textarea>
          </label>
        </fieldset>
      {/each}
    </div>
  {/if}
</section>
