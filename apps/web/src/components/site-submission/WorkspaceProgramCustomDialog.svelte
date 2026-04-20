<script lang="ts">
  import type { SiteSubmissionOptionItem } from '@/application/site-submission/site-submission.service';
  import ModalSurface from '@/shared/ui/ModalSurface.svelte';

  import SiteTechStackCombobox from './SiteTechStackCombobox.svelte';

  let {
    open = $bindable(false),
    programName = $bindable(''),
    programOpenSource = $bindable<boolean | null>(null),
    websiteUrl = $bindable(''),
    repoUrl = $bindable(''),
    frameworkIds = $bindable<string[]>([]),
    frameworkCustomNames = $bindable<string[]>([]),
    languageIds = $bindable<string[]>([]),
    languageCustomNames = $bindable<string[]>([]),
    techStackOptions = [],
    error = '',
    onCancel,
    onConfirm,
    onProgramNameInput,
  }: {
    open?: boolean;
    programName?: string;
    programOpenSource?: boolean | null;
    websiteUrl?: string;
    repoUrl?: string;
    frameworkIds?: string[];
    frameworkCustomNames?: string[];
    languageIds?: string[];
    languageCustomNames?: string[];
    techStackOptions?: SiteSubmissionOptionItem[];
    error?: string;
    onCancel: (() => void) | undefined;
    onConfirm: (() => void) | undefined;
    onProgramNameInput: ((value: string) => void) | undefined;
  } = $props();

  let inputElement: HTMLInputElement | null = null;

  $effect(() => {
    if (!open) {
      return;
    }

    inputElement?.focus();
    inputElement?.select();
  });

  const openSourceSelectValue = (value: boolean | null): string => {
    if (value === true) {
      return 'true';
    }

    if (value === false) {
      return 'false';
    }

    return '';
  };
</script>

<ModalSurface
  {open}
  title="使用自定义程序"
  description="在这里完整填写自定义架构。程序名必填，开源情况、官网、仓库和技术栈都可选。"
  confirmLabel="确认使用"
  cancelLabel="取消"
  {onCancel}
  {onConfirm}
>
  <div class="space-y-4">
    <label class="block space-y-2 text-sm" for="submission-custom-program-input">
      <span class="block font-medium text-(--color-fg)">程序名称</span>
      <input
        id="submission-custom-program-input"
        bind:this={inputElement}
        bind:value={programName}
        class="min-h-11 w-full rounded-md border border-(--color-line-med) bg-(--color-bg-raised) px-3 py-2 text-sm text-(--color-fg) outline-none placeholder:text-(--color-fg-3) focus:border-red-700/35 dark:focus:border-red-400/35"
        placeholder="例如：WordPress、Astro、自研博客系统"
        maxlength="128"
        oninput={(event) => onProgramNameInput?.((event.currentTarget as HTMLInputElement).value)}
        onkeydown={(event) => {
          if (event.key === 'Enter') {
            event.preventDefault();
            onConfirm?.();
          }
        }}
      />
    </label>

    <div class="grid gap-4 md:grid-cols-2">
      <label class="block space-y-2 text-sm" for="submission-custom-program-open-source">
        <span class="block font-medium text-(--color-fg)">开源情况</span>
        <select
          id="submission-custom-program-open-source"
          class="min-h-11 w-full rounded-md border border-(--color-line-med) bg-(--color-bg-raised) px-3 py-2 text-sm text-(--color-fg) outline-none focus:border-red-700/35 dark:focus:border-red-400/35"
          value={openSourceSelectValue(programOpenSource)}
          onchange={(event) => {
            const value = (event.currentTarget as HTMLSelectElement).value;
            programOpenSource = value === 'true' ? true : value === 'false' ? false : null;
          }}
        >
          <option value="">暂不说明</option>
          <option value="true">是，开源程序</option>
          <option value="false">否，非开源或暂不公开</option>
        </select>
      </label>
      <label class="block space-y-2 text-sm" for="submission-custom-program-website">
        <span class="block font-medium text-(--color-fg)">官网地址</span>
        <input
          id="submission-custom-program-website"
          bind:value={websiteUrl}
          class="min-h-11 w-full rounded-md border border-(--color-line-med) bg-(--color-bg-raised) px-3 py-2 text-sm text-(--color-fg) outline-none placeholder:text-(--color-fg-3) focus:border-red-700/35 dark:focus:border-red-400/35"
          placeholder="https://example.com"
          inputmode="url"
        />
      </label>
    </div>

    <label class="block space-y-2 text-sm" for="submission-custom-program-repo">
      <span class="block font-medium text-(--color-fg)">仓库地址</span>
      <input
        id="submission-custom-program-repo"
        bind:value={repoUrl}
        class="min-h-11 w-full rounded-md border border-(--color-line-med) bg-(--color-bg-raised) px-3 py-2 text-sm text-(--color-fg) outline-none placeholder:text-(--color-fg-3) focus:border-red-700/35 dark:focus:border-red-400/35"
        placeholder="https://github.com/example/project"
        inputmode="url"
      />
    </label>

    <div class="space-y-2">
      <span class="block text-sm font-medium text-(--color-fg)">技术栈</span>
      <SiteTechStackCombobox
        inputId="submission-custom-program-tech-stacks"
        options={techStackOptions}
        bind:frameworkIds
        bind:frameworkCustomNames
        bind:languageIds
        bind:languageCustomNames
      />
      <p class="text-xs leading-6 text-(--color-fg-3)">
        在一个下拉框里统一选择或补充技术栈；如果没有单独的官网地址，提交时会自动将仓库地址作为官网地址。
      </p>
    </div>

    {#if error}
      <p class="text-xs text-(--color-fail)">{error}</p>
    {/if}
  </div>
</ModalSurface>
