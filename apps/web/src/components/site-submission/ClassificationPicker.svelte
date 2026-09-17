<script lang="ts">
  import type { EditableSubmission } from '@/application/site-submission/site-submission.browser';
  import {
    removeTag,
    selectClassificationTag,
  } from '@/application/site-submission/site-submission.tags.browser';
  import type { Option } from '@/application/site-submission/site-submission.types';

  interface Props {
    form: EditableSubmission;
    options: readonly Option[];
  }

  let { form = $bindable(), options }: Props = $props();
  const level1Options = $derived(
    options.filter((option) => option.level === 1 && option.role !== 'WARNING'),
  );
  const selectedLevel1 = $derived(form.tags.find((tag) => tag.level === 1));
  const selectedLevel2 = $derived(form.tags.find((tag) => tag.level === 2));
  const level2Options = $derived(
    options.filter(
      (option) =>
        option.level === 2 && option.role !== 'WARNING' && option.parent_id === selectedLevel1?.id,
    ),
  );

  function changeLevel1(event: Event): void {
    const value = event.currentTarget;
    if (!(value instanceof HTMLSelectElement)) return;
    if (!value.value) {
      if (selectedLevel1) removeTag(form, selectedLevel1.id);
      return;
    }
    const option = level1Options.find((candidate) => candidate.id === value.value);
    if (option) selectClassificationTag(form, option);
  }

  function changeLevel2(event: Event): void {
    const value = event.currentTarget;
    if (!(value instanceof HTMLSelectElement)) return;
    if (!value.value) {
      if (selectedLevel2) removeTag(form, selectedLevel2.id);
      return;
    }
    const option = level2Options.find((candidate) => candidate.id === value.value);
    if (option) selectClassificationTag(form, option);
  }
</script>

<fieldset class="grid min-w-0 gap-3">
  <legend class="text-sm font-semibold">固定分类</legend>
  <div class="grid min-w-0 gap-4 sm:grid-cols-2">
    <label class="grid min-w-0 gap-1.5 text-sm">
      一级分类
      <select
        class="min-h-11 min-w-0 rounded-md border border-line-strong bg-surface px-3 sm:min-h-10"
        value={selectedLevel1?.id ?? ''}
        onchange={changeLevel1}
      >
        <option value="">请选择一级分类</option>
        {#each level1Options as option (option.id)}
          <option value={option.id}>{option.name}</option>
        {/each}
      </select>
    </label>
    <label class="grid min-w-0 gap-1.5 text-sm">
      二级分类
      <select
        class="min-h-11 min-w-0 rounded-md border border-line-strong bg-surface px-3 disabled:cursor-not-allowed disabled:border-line disabled:bg-subtle disabled:text-fg-muted sm:min-h-10"
        value={selectedLevel2?.id ?? ''}
        disabled={!selectedLevel1}
        onchange={changeLevel2}
      >
        <option value="">{selectedLevel1 ? '请选择二级分类' : '请先选择一级分类'}</option>
        {#each level2Options as option (option.id)}
          <option value={option.id}>{option.name}</option>
        {/each}
      </select>
    </label>
  </div>
</fieldset>
