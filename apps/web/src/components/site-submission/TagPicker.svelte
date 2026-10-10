<script lang="ts">
  import type { Option } from '@/api/site-submission/site-submission.types';
  import type { EditableSubmission } from '@/application/site-submission/site-submission.browser';

  import ClassificationPicker from './ClassificationPicker.svelte';
  import TertiaryTagPicker from './TertiaryTagPicker.svelte';

  interface Props {
    form: EditableSubmission;
    options: readonly Option[];
    canManageTaxonomy?: boolean;
    disabled?: boolean;
  }

  let {
    form = $bindable(),
    options,
    canManageTaxonomy = false,
    disabled = false,
  }: Props = $props();
  const customTags = $derived(form.tags.filter((tag) => tag.suggestedName));
</script>

<section class="grid min-w-0 gap-5">
  <div>
    <h2 class="text-xl font-semibold">标签</h2>
    <p class="mt-1 text-sm text-pretty text-fg-muted">选择固定分类，并按需添加三级标签。</p>
  </div>

  <fieldset class="grid min-w-0 gap-5" {disabled}>
    <ClassificationPicker bind:form {options} />

    <div class="border-t border-line pt-5">
      <TertiaryTagPicker bind:form {options} />
    </div>
  </fieldset>

  {#if canManageTaxonomy && customTags.length > 0}
    <div class="grid gap-4 border-t border-line pt-5">
      {#each customTags as tag (tag.id)}
        <fieldset class="grid min-w-0 gap-3 rounded-sm border border-line p-3" {disabled}>
          <legend class="max-w-full px-1 text-sm font-semibold wrap-anywhere">{tag.name}</legend>
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
