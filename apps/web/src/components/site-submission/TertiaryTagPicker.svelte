<script lang="ts">
  import { IconPlus, IconSearch, IconX } from '@tabler/icons-svelte';
  import { tick } from 'svelte';

  import type { EditableSubmission } from '@/application/site-submission/site-submission.browser';
  import { nextDraftID } from '@/application/site-submission/site-submission.draft-id.browser';
  import { matchesSubmissionOption } from '@/application/site-submission/site-submission.search';
  import {
    removeTag,
    selectTertiaryTag,
    tertiaryTagOptions,
  } from '@/application/site-submission/site-submission.tags.browser';
  import type { Option } from '@/application/site-submission/site-submission.types';

  interface Props {
    form: EditableSubmission;
    options: readonly Option[];
  }

  let { form = $bindable(), options }: Props = $props();
  const pickerID = $props.id();
  let query = $state('');
  let open = $state(false);
  let activeIndex = $state(0);
  let picker: HTMLElement;
  const selected = $derived(form.tags.filter((tag) => tag.role === 'TERTIARY'));
  const filtered = $derived(
    tertiaryTagOptions(options, form.tags).filter((option) =>
      matchesSubmissionOption(query, option.name),
    ),
  );
  const normalizedQuery = $derived(query.trim().toLocaleLowerCase());
  const canSuggest = $derived(
    selected.length < 20 &&
      normalizedQuery.length > 0 &&
      query.trim().length <= 100 &&
      !options.some((option) => option.name.trim().toLocaleLowerCase() === normalizedQuery) &&
      !form.tags.some((tag) => tag.name.trim().toLocaleLowerCase() === normalizedQuery),
  );

  const optionCount = $derived(filtered.length + Number(canSuggest));
  const activeOptionID = $derived(
    activeIndex < filtered.length
      ? `${pickerID}-option-${filtered[activeIndex]?.id}`
      : canSuggest
        ? `${pickerID}-custom`
        : undefined,
  );

  function choose(option: Option): void {
    selectTertiaryTag(form, option);
    query = '';
    activeIndex = 0;
  }

  function chooseCustom(): void {
    if (!canSuggest) return;
    selectTertiaryTag(form, {
      id: nextDraftID('tag'),
      name: query.trim(),
      level: 3,
      is_custom: true,
    });
    query = '';
    activeIndex = 0;
  }

  async function keydown(event: KeyboardEvent): Promise<void> {
    if (event.isComposing) return;
    if (event.key === 'Escape') {
      open = false;
      return;
    }
    if (
      event.key === 'ArrowDown' ||
      event.key === 'ArrowUp' ||
      (open && (event.key === 'Home' || event.key === 'End'))
    ) {
      event.preventDefault();
      const lastIndex = Math.max(optionCount - 1, 0);
      if (event.key === 'Home') activeIndex = 0;
      else if (event.key === 'End') activeIndex = lastIndex;
      else if (!open) activeIndex = event.key === 'ArrowDown' ? 0 : lastIndex;
      else
        activeIndex = Math.max(
          0,
          Math.min(activeIndex + (event.key === 'ArrowDown' ? 1 : -1), lastIndex),
        );
      open = true;
      await tick();
      if (activeOptionID)
        document.getElementById(activeOptionID)?.scrollIntoView({ block: 'nearest' });
      return;
    }
    if (event.key !== 'Enter' || !open) return;
    event.preventDefault();
    const option = filtered[activeIndex];
    if (option) choose(option);
    else chooseCustom();
  }

  function closeOutside(event: Event): void {
    if (event.target instanceof Node && !picker.contains(event.target)) open = false;
  }
</script>

<svelte:document onclick={closeOutside} onfocusin={closeOutside} />

<div class="grid min-w-0 gap-3" bind:this={picker}>
  <div class="flex items-end justify-between gap-3">
    <div>
      <h3 class="text-sm font-semibold">三级标签</h3>
      <p class="mt-1 text-xs text-fg-muted">可多选，也可输入新标签。</p>
    </div>
    <span class="shrink-0 font-mono text-xs text-fg-muted">{selected.length} / 20</span>
  </div>

  <div class="relative min-w-0">
    <label class="sr-only" for={`${pickerID}-search`}>搜索或新增三级标签</label>
    <IconSearch
      class="pointer-events-none absolute top-3.5 left-3 text-fg-muted sm:top-3"
      size={18}
      aria-hidden="true"
    />
    <input
      id={`${pickerID}-search`}
      class="min-h-11 w-full min-w-0 rounded-md border border-line-strong bg-surface pr-3 pl-10 sm:min-h-10"
      bind:value={query}
      role="combobox"
      aria-expanded={open}
      aria-controls={`${pickerID}-options`}
      aria-autocomplete="list"
      aria-activedescendant={open ? activeOptionID : undefined}
      autocomplete="off"
      maxlength="100"
      placeholder="搜索或输入新标签"
      onfocus={() => (open = true)}
      oninput={() => {
        open = true;
        activeIndex = 0;
      }}
      onkeydown={keydown}
    />
    {#if open}
      <div
        id={`${pickerID}-options`}
        class="absolute z-20 mt-1 max-h-72 w-full min-w-0 overflow-y-auto overscroll-contain rounded-md border border-line bg-surface p-1 shadow-sm"
        role="listbox"
        aria-label="可选三级标签"
        aria-multiselectable="true"
      >
        {#each filtered as option, index (option.id)}
          <button
            id={`${pickerID}-option-${option.id}`}
            class="flex min-h-11 w-full min-w-0 items-center gap-2 rounded-sm px-3 text-left text-sm hover:bg-subtle focus-visible:bg-subtle disabled:cursor-not-allowed disabled:opacity-50 sm:min-h-10"
            class:bg-subtle={index === activeIndex}
            type="button"
            role="option"
            aria-selected="false"
            tabindex="-1"
            disabled={selected.length >= 20}
            onmousedown={(event) => event.preventDefault()}
            onmouseenter={() => (activeIndex = index)}
            onclick={() => choose(option)}
          >
            <span class="min-w-0 flex-1 wrap-anywhere">{option.name}</span>
          </button>
        {/each}
        {#if canSuggest}
          <button
            id={`${pickerID}-custom`}
            type="button"
            class="flex min-h-11 w-full min-w-0 items-center gap-2 rounded-sm px-3 text-left text-sm text-tint-fg hover:bg-tint sm:min-h-10"
            role="option"
            tabindex="-1"
            aria-selected="false"
            class:bg-tint={activeIndex === filtered.length}
            onmousedown={(event) => event.preventDefault()}
            onmouseenter={() => (activeIndex = filtered.length)}
            onclick={chooseCustom}
          >
            <IconPlus class="shrink-0" size={16} aria-hidden="true" />
            <span class="min-w-0 wrap-anywhere">新增“{query.trim()}”</span>
          </button>
        {/if}
        {#if filtered.length === 0 && !canSuggest}
          <p class="p-3 text-sm text-fg-muted">
            {selected.length >= 20 ? '已达到 20 个三级标签上限' : '没有可选标签'}
          </p>
        {/if}
      </div>
    {/if}
  </div>

  {#if selected.length > 0}
    <div
      class="flex max-h-36 min-w-0 flex-wrap gap-2 overflow-y-auto rounded-md border border-line bg-subtle p-2"
      role="group"
      aria-label="已选三级标签"
    >
      {#each selected as tag (tag.id)}
        <span
          class="inline-flex min-h-11 max-w-full items-center rounded-sm bg-tint pl-2.5 text-sm text-tint-fg sm:min-h-10"
        >
          <span class="min-w-0 wrap-anywhere">{tag.name}</span>
          <button
            class="inline-flex size-11 shrink-0 items-center justify-center rounded-sm hover:bg-primary/10 sm:size-10"
            type="button"
            aria-label={`移除三级标签：${tag.name}`}
            onclick={() => removeTag(form, tag.id)}
          >
            <IconX size={14} aria-hidden="true" />
          </button>
        </span>
      {/each}
    </div>
  {/if}
</div>
