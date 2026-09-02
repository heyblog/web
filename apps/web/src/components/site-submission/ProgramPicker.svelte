<script lang="ts">
  import { tick, untrack } from 'svelte';

  import type {
    EditableSubmission,
    ProgramDraft,
  } from '@/application/site-submission/site-submission.browser';
  import { matchesSubmissionOption } from '@/application/site-submission/site-submission.search';
  import type {
    ComponentOption,
    ProgramDependencyOption,
  } from '@/application/site-submission/site-submission.types';

  import TechnologyPicker from './TechnologyPicker.svelte';

  interface Props {
    form: EditableSubmission;
    options: readonly ComponentOption[];
    dependencyRelations: readonly ProgramDependencyOption[];
    privateProgramID: string;
  }

  type PickerMode = 'search' | 'custom' | 'other';
  type CustomProgramDraft = Extract<ProgramDraft, { kind: 'custom' }>;

  let { form, options, dependencyRelations, privateProgramID }: Props = $props();
  const initialProgram = untrack(() => form.program);
  let query = $state('');
  let searchInput: HTMLInputElement;
  let mode = $state<PickerMode>(
    initialProgram.kind === 'custom'
      ? 'custom'
      : initialProgram.kind === 'other'
        ? 'other'
        : 'search',
  );
  let customDraft = $state<CustomProgramDraft>(
    initialProgram.kind === 'custom'
      ? cloneCustom(initialProgram)
      : {
          kind: 'custom',
          name: '',
          isOpenSource: null,
          homepageURL: '',
          repositoryURL: '',
          dependencies: [],
        },
  );
  let filtered = $derived(
    options.filter(
      (option) => option.id !== privateProgramID && matchesSubmissionOption(query, option.name),
    ),
  );

  function cloneCustom(program: CustomProgramDraft): CustomProgramDraft {
    return { ...program, dependencies: program.dependencies.map((item) => ({ ...item })) };
  }

  function rememberCustom(): void {
    if (form.program.kind === 'custom') customDraft = cloneCustom(form.program);
  }

  function chooseOther(): void {
    rememberCustom();
    const option = options.find((item) => item.id === privateProgramID);
    if (!option) return;
    form.program = { kind: 'other', id: option.id, name: '其他' };
    mode = 'other';
  }

  function chooseExisting(option: ComponentOption): void {
    const dependencies = dependencyRelations
      .filter(
        (item) =>
          item.program_id === option.id && (item.role === 'FRAMEWORK' || item.role === 'LANGUAGE'),
      )
      .map((item) => {
        const component = options.find((candidate) => candidate.id === item.component_id);
        return {
          id: item.component_id,
          name: component?.name ?? item.component_id,
          role: item.role === 'FRAMEWORK' ? ('FRAMEWORK' as const) : ('LANGUAGE' as const),
        };
      });
    form.program = { kind: 'existing', id: option.id, name: option.name, dependencies };
    query = '';
  }

  function chooseCustom(): void {
    if (!customDraft.name.trim() && query.trim()) customDraft.name = query.trim();
    form.program = cloneCustom(customDraft);
    mode = 'custom';
  }

  async function returnToSearch(): Promise<void> {
    rememberCustom();
    form.program = { kind: 'none' };
    mode = 'search';
    await tick();
    searchInput.focus();
  }

  function setOpenSource(value: string): void {
    if (form.program.kind === 'custom')
      form.program.isOpenSource = value === '' ? null : value === 'true';
  }
</script>

<section class="grid gap-4">
  <div>
    <h2 class="text-xl font-semibold">站点程序</h2>
    <p class="mt-1 text-sm text-pretty text-fg-muted">搜索已有程序，或选择程序类型。</p>
  </div>

  <div class="grid items-start gap-4 sm:grid-cols-[10rem_minmax(0,1fr)]">
    <div class="grid gap-2" aria-label="程序类型">
      <button
        class="min-h-11 rounded-sm border px-3 text-sm"
        class:border-primary={mode === 'custom'}
        class:bg-tint={mode === 'custom'}
        class:border-line={mode !== 'custom'}
        type="button"
        aria-pressed={mode === 'custom'}
        onclick={chooseCustom}>自定义程序</button
      >
      <button
        class="min-h-11 rounded-sm border px-3 text-sm"
        class:border-primary={mode === 'other'}
        class:bg-tint={mode === 'other'}
        class:border-line={mode !== 'other'}
        type="button"
        aria-pressed={mode === 'other'}
        disabled={!privateProgramID}
        onclick={chooseOther}>其他</button
      >
    </div>

    <div class="grid gap-3">
      <label class="grid gap-2 text-sm" for="program-search"
        >选择已有程序<input
          id="program-search"
          class="min-h-11 rounded-sm border border-line-strong bg-surface px-3 disabled:cursor-not-allowed disabled:border-line disabled:bg-subtle disabled:text-fg-muted"
          bind:this={searchInput}
          bind:value={query}
          disabled={mode !== 'search'}
          autocomplete="off"
          placeholder="WordPress、Typecho、Astro…"
        /></label
      >

      {#if mode === 'search'}
        {#if query.trim()}<div
            class="grid max-h-80 overflow-y-auto overscroll-contain rounded-sm border border-line p-1"
          >
            {#each filtered as option (option.id)}<button
                class="min-h-11 rounded-sm px-3 text-left text-sm hover:bg-subtle"
                type="button"
                onclick={() => chooseExisting(option)}>{option.name}</button
              >{/each}
            {#if filtered.length === 0}<p class="p-3 text-sm text-fg-muted">
                没有匹配的站点程序
              </p>{/if}
          </div>{/if}
        {#if form.program.kind === 'existing'}<div
            class="rounded-sm border border-line bg-subtle p-4"
          >
            <p class="font-semibold">{form.program.name}</p>
            {#if form.program.dependencies.length}<p class="mt-2 text-sm text-fg-muted">
                目录技术栈：{form.program.dependencies
                  .map((item) => `${item.name}（${item.role === 'FRAMEWORK' ? '框架' : '语言'}）`)
                  .join('、')}
              </p>{/if}
          </div>{/if}
      {:else if form.program.kind === 'custom'}
        <div class="grid gap-4 rounded-md border border-line p-4">
          <div class="flex flex-wrap items-center justify-between gap-3 border-b border-line pb-3">
            <strong>自定义程序</strong>
            <button
              class="min-h-10 rounded-sm border border-line-strong px-3 text-sm font-medium hover:bg-subtle"
              type="button"
              onclick={returnToSearch}>返回选择已有程序</button
            >
          </div>
          <div class="grid gap-4 sm:grid-cols-2">
            <label class="grid gap-2 text-sm"
              >程序名称<input
                class="min-h-11 rounded-sm border border-line-strong bg-surface px-3"
                bind:value={form.program.name}
                maxlength="128"
              /></label
            >
            <label class="grid gap-2 text-sm"
              >开源状态<select
                class="min-h-11 rounded-sm border border-line-strong bg-surface px-3"
                value={form.program.isOpenSource === null ? '' : String(form.program.isOpenSource)}
                onchange={(event) => setOpenSource(event.currentTarget.value)}
                ><option value="">请选择</option><option value="true">开源</option><option
                  value="false">不开源</option
                ></select
              ></label
            >
            <label class="grid gap-2 text-sm"
              >官网<input
                class="min-h-11 rounded-sm border border-line-strong bg-surface px-3"
                bind:value={form.program.homepageURL}
              /></label
            >
            <label class="grid gap-2 text-sm"
              >代码仓库<input
                class="min-h-11 rounded-sm border border-line-strong bg-surface px-3"
                bind:value={form.program.repositoryURL}
              /></label
            >
          </div>
          <TechnologyPicker
            bind:dependencies={form.program.dependencies}
            {options}
            role="FRAMEWORK"
          />
          <TechnologyPicker
            bind:dependencies={form.program.dependencies}
            {options}
            role="LANGUAGE"
          />
        </div>
      {:else}
        <div class="grid gap-3 rounded-md border border-line bg-subtle p-4">
          <strong>已选择其他</strong>
          <button
            class="min-h-10 w-fit rounded-sm border border-line-strong bg-surface px-3 text-sm font-medium hover:bg-subtle"
            type="button"
            onclick={returnToSearch}>返回选择已有程序</button
          >
        </div>
      {/if}
    </div>
  </div>
</section>
