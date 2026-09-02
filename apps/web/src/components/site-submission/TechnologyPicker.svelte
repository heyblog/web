<script lang="ts">
  import { IconPlus, IconX } from '@tabler/icons-svelte';

  import type { DependencyDraft } from '@/application/site-submission/site-submission.browser';
  import { matchesSubmissionOption } from '@/application/site-submission/site-submission.search';
  import type {
    ComponentOption,
    DependencyRole,
  } from '@/application/site-submission/site-submission.types';
  interface Props {
    dependencies: DependencyDraft[];
    options: readonly ComponentOption[];
    role: DependencyRole;
  }
  let { dependencies = $bindable(), options, role }: Props = $props();
  let query = $state('');
  let filtered = $derived(
    options
      .filter(
        (option) =>
          !dependencies.some((item) => item.id === option.id && item.role === role) &&
          matchesSubmissionOption(query, option.name),
      )
      .slice(0, 8),
  );
  function addExisting(option: ComponentOption): void {
    dependencies.push({ id: option.id, name: option.name, role });
    query = '';
  }
  function addCustom(): void {
    const name = query.trim();
    if (
      !name ||
      dependencies.some(
        (item) =>
          item.role === role &&
          item.name.toLocaleLowerCase('zh-CN') === name.toLocaleLowerCase('zh-CN'),
      )
    )
      return;
    dependencies.push({
      id: '',
      name,
      role,
      isOpenSource: null,
      homepageURL: '',
      repositoryURL: '',
    });
    query = '';
  }
  function remove(index: number): void {
    dependencies.splice(index, 1);
  }
</script>

<div class="grid gap-3">
  <label class="grid gap-2 text-sm"
    >{role === 'FRAMEWORK' ? '框架' : '语言'}<input
      class="min-h-11 rounded-sm border border-line-strong bg-surface px-3"
      bind:value={query}
      placeholder="搜索已有项，或输入自定义名称"
    /></label
  >
  {#if query.trim()}<div class="grid rounded-sm border border-line p-1">
      {#each filtered as option (option.id)}<button
          class="min-h-11 rounded-sm px-3 text-left text-sm hover:bg-subtle"
          type="button"
          onclick={() => addExisting(option)}>{option.name}</button
        >{/each}
      <button
        class="inline-flex min-h-11 items-center gap-2 rounded-sm px-3 text-left text-sm text-tint-fg hover:bg-tint"
        type="button"
        onclick={addCustom}><IconPlus size={16} aria-hidden="true" />添加“{query.trim()}”</button
      >
    </div>{/if}
  <div class="grid gap-2">
    {#each dependencies as dependency, index (`${dependency.role}:${dependency.id}:${dependency.name}`)}{#if dependency.role === role}<div
          class="grid gap-3 rounded-sm border border-line bg-subtle p-3 text-sm"
        >
          <span class="flex min-h-10 items-center justify-between gap-2"
            ><strong>{dependency.name}</strong><button
              class="inline-flex size-10 items-center justify-center"
              type="button"
              aria-label={`移除 ${dependency.name}`}
              onclick={() => remove(index)}><IconX size={16} aria-hidden="true" /></button
            ></span
          >
          {#if !dependency.id}<div class="grid gap-3 sm:grid-cols-2">
              <label class="grid gap-2 text-xs"
                >主页<input
                  class="min-h-11 rounded-sm border border-line-strong bg-surface px-3 text-sm"
                  bind:value={dependency.homepageURL}
                /></label
              >
              <label class="grid gap-2 text-xs"
                >代码仓库<input
                  class="min-h-11 rounded-sm border border-line-strong bg-surface px-3 text-sm"
                  bind:value={dependency.repositoryURL}
                /></label
              >
              <label class="grid gap-2 text-xs sm:col-span-2"
                >开源状态<select
                  class="min-h-11 rounded-sm border border-line-strong bg-surface px-3 text-sm"
                  value={dependency.isOpenSource === null
                    ? ''
                    : String(dependency.isOpenSource ?? '')}
                  onchange={(event) =>
                    (dependency.isOpenSource =
                      event.currentTarget.value === ''
                        ? null
                        : event.currentTarget.value === 'true')}
                  ><option value="">请选择</option><option value="true">开源</option><option
                    value="false">不开源</option
                  ></select
                ></label
              >
            </div>{/if}
        </div>{/if}{/each}
  </div>
</div>
