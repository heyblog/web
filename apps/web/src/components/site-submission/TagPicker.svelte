<script lang="ts">
  import type { EditableSubmission } from '@/application/site-submission/site-submission.browser';
  import type { Option } from '@/application/site-submission/site-submission.types';

  import ClassificationPicker from './ClassificationPicker.svelte';
  import TertiaryTagPicker from './TertiaryTagPicker.svelte';

  interface Props {
    form: EditableSubmission;
    options: readonly Option[];
    canManageTaxonomy?: boolean;
  }

  let { form = $bindable(), options, canManageTaxonomy = false }: Props = $props();
  const customTags = $derived(form.tags.filter((tag) => tag.suggestedName));
</script>

<section class="grid min-w-0 gap-5">
  <div>
    <h2 class="text-xl font-semibold">标签</h2>
    <p class="mt-1 text-sm text-pretty text-fg-muted">选择固定分类，并按需添加三级标签。</p>
  </div>

  <ClassificationPicker bind:form {options} />

  <div class="border-t border-line pt-5">
    <TertiaryTagPicker bind:form {options} />
  </div>

  {#if canManageTaxonomy && customTags.length > 0}
    <div class="grid gap-4 border-t border-line pt-5">
      <p class="text-sm text-fg-muted">批准新增三级标签前，请补全 slug 和说明。</p>
      {#each customTags as tag (tag.id)}
        <fieldset class="grid min-w-0 gap-3 rounded-sm border border-line p-3">
          <legend class="max-w-full px-1 text-sm font-semibold wrap-anywhere">{tag.name}</legend>
          <label class="grid min-w-0 gap-1.5 text-sm">
            Slug
            <input
              class="min-h-11 min-w-0 rounded-sm border border-line-strong bg-surface px-3 sm:min-h-10"
              bind:value={tag.slug}
              pattern="[a-z0-9]+(?:-[a-z0-9]+)*"
              placeholder="例如：independent-writing"
            />
          </label>
          <label class="grid min-w-0 gap-1.5 text-sm">
            标签说明
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
