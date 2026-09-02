<script lang="ts">
  import { IconSearch, IconX } from '@tabler/icons-svelte';

  import {
    type EditableSubmission,
    makePrimaryTag,
    removeTag,
    selectTag,
  } from '@/application/site-submission/site-submission.browser';
  import { matchesSubmissionOption } from '@/application/site-submission/site-submission.search';
  import type { Option } from '@/application/site-submission/site-submission.types';
  interface Props {
    form: EditableSubmission;
    options: readonly Option[];
  }
  let { form, options }: Props = $props();
  let query = $state('');
  let open = $state(false);
  let activeIndex = $state(0);
  let picker: HTMLElement;
  let filtered = $derived(
    options.filter(
      (option) =>
        !form.tags.some((tag) => tag.id === option.id) &&
        matchesSubmissionOption(query, option.name),
    ),
  );
  function choose(option: Option): void {
    selectTag(form, option);
    query = '';
    open = false;
    activeIndex = 0;
  }
  function keydown(event: KeyboardEvent): void {
    if (event.key === 'Escape') {
      open = false;
      return;
    }
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      activeIndex = Math.min(activeIndex + 1, filtered.length - 1);
      open = true;
      return;
    }
    if (event.key === 'ArrowUp') {
      event.preventDefault();
      activeIndex = Math.max(activeIndex - 1, 0);
      open = true;
      return;
    }
    if (event.key === 'Home') {
      event.preventDefault();
      activeIndex = 0;
      return;
    }
    if (event.key === 'End') {
      event.preventDefault();
      activeIndex = Math.max(filtered.length - 1, 0);
      return;
    }
    if (event.key === 'Enter' && filtered[activeIndex]) {
      event.preventDefault();
      choose(filtered[activeIndex]);
    }
  }
  function documentClick(event: MouseEvent): void {
    if (event.target instanceof Node && !picker.contains(event.target)) open = false;
  }
</script>

<svelte:document onclick={documentClick} />

<section class="grid gap-4" bind:this={picker}>
  <div>
    <h2 class="text-xl font-semibold">标签</h2>
    <p class="mt-1 text-sm text-fg-muted">选择 1–12 个已有标签，并指定一个主标签。</p>
  </div>
  <div class="relative">
    <label class="grid gap-2 text-sm" for="tag-search">搜索标签</label>
    <div class="relative mt-2">
      <IconSearch
        class="pointer-events-none absolute top-3.5 left-3 text-fg-muted"
        size={18}
        aria-hidden="true"
      /><input
        id="tag-search"
        class="min-h-11 w-full rounded-sm border border-line-strong bg-surface pr-3 pl-10"
        bind:value={query}
        role="combobox"
        aria-expanded={open}
        aria-controls="tag-results"
        aria-activedescendant={open && filtered[activeIndex]
          ? `tag-option-${filtered[activeIndex].id}`
          : undefined}
        autocomplete="off"
        onfocus={() => (open = true)}
        oninput={() => {
          open = true;
          activeIndex = 0;
        }}
        onkeydown={keydown}
      />
    </div>
    {#if open}<div
        id="tag-results"
        class="absolute z-20 mt-1 max-h-80 w-full overflow-y-auto overscroll-contain rounded-md border border-line bg-surface p-1 shadow-sm"
        role="listbox"
      >
        {#each filtered as option, index (option.id)}<button
            id={`tag-option-${option.id}`}
            class="flex min-h-11 w-full items-center rounded-sm px-3 text-left text-sm hover:bg-subtle focus-visible:bg-subtle"
            class:bg-subtle={index === activeIndex}
            type="button"
            role="option"
            aria-selected={index === activeIndex}
            onmousedown={(event) => event.preventDefault()}
            onmouseenter={() => (activeIndex = index)}
            onclick={() => choose(option)}>{option.name}</button
          >{/each}
        {#if filtered.length === 0}<p class="p-3 text-sm text-fg-muted">没有可选标签</p>{/if}
      </div>{/if}
  </div>
  <div class="flex flex-wrap gap-2" role="group" aria-label="已选标签">
    {#each form.tags as tag (tag.id)}<span
        class="inline-flex min-h-11 items-center gap-1 rounded-sm border border-line bg-tint pl-3 text-sm text-tint-fg"
      >
        <button
          class="min-h-11 font-medium"
          type="button"
          aria-pressed={tag.role === 'PRIMARY'}
          onclick={() => makePrimaryTag(form, tag.id)}
          >{tag.name}{tag.role === 'PRIMARY' ? ' · 主标签' : ''}</button
        >
        <button
          class="inline-flex size-11 items-center justify-center"
          type="button"
          aria-label={`移除 ${tag.name}`}
          onclick={() => removeTag(form, tag.id)}><IconX size={16} aria-hidden="true" /></button
        >
      </span>{/each}
  </div>
</section>
