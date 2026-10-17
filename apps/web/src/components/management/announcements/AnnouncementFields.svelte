<script lang="ts">
  import type { AnnouncementDraft } from '../../../application/announcements/announcement-editor.shared.ts';
  import type {
    AnnouncementField as FieldName,
    AnnouncementFieldErrors,
  } from '../../../application/announcements/announcement-validation.shared.ts';

  import AnnouncementField from './AnnouncementField.svelte';

  let {
    draft = $bindable(),
    identityLocked = false,
    disabled = false,
    timeZone = '',
    fieldErrors = {},
    attempted = false,
  }: {
    draft: AnnouncementDraft;
    identityLocked?: boolean;
    disabled?: boolean;
    timeZone?: string;
    fieldErrors?: AnnouncementFieldErrors;
    attempted?: boolean;
  } = $props();
  let touched = $state<Partial<Record<FieldName, boolean>>>({});
  const inputBaseClass =
    'min-h-11 w-full rounded-md border bg-surface px-3 text-sm transition-colors duration-(--motion-color) disabled:bg-subtle disabled:text-fg-muted sm:min-h-10';
  function inputClass(error?: string) {
    return `${inputBaseClass} ${error ? 'border-danger-solid' : 'border-line-strong'}`;
  }
  function issue(field: FieldName) {
    return attempted || touched[field] ? fieldErrors[field] : undefined;
  }
</script>

<fieldset {disabled} class="grid min-w-0 gap-6">
  <div class="grid gap-4 sm:grid-cols-2">
    <AnnouncementField label="公告类型">
      {#snippet children(id)}<select
          {id}
          class={inputClass()}
          bind:value={draft.kind}
          disabled={identityLocked}
        >
          <option value="MAIN">主公告</option><option value="BANNER">横幅公告</option>
        </select>{/snippet}
    </AnnouncementField>
    {#if draft.kind === 'MAIN'}
      <AnnouncementField
        label="优先级"
        validated
        error={issue('priority')}
        help="整数；数值越大，排序越靠前。"
      >
        {#snippet children(id, error)}<input
            {id}
            class={inputClass(error)}
            type="number"
            min="-2147483648"
            max="2147483647"
            step="1"
            bind:value={draft.priority}
            required
            aria-invalid={error ? true : undefined}
            aria-describedby={`${id}-help${error ? ` ${id}-error` : ''}`}
            onblur={() => {
              touched.priority = true;
            }}
          />{/snippet}
      </AnnouncementField>
    {/if}
  </div>
  <AnnouncementField label="标题" validated error={issue('title')} help="必填，最多 256 个字符。">
    {#snippet children(id, error)}<input
        {id}
        class={inputClass(error)}
        bind:value={draft.title}
        required
        aria-invalid={error ? true : undefined}
        aria-describedby={`${id}-help${error ? ` ${id}-error` : ''}`}
        onblur={() => {
          touched.title = true;
        }}
      />{/snippet}
  </AnnouncementField>
  <AnnouncementField label="正文">
    {#snippet children(id)}<textarea
        {id}
        class={`${inputClass()} min-h-40 py-3 font-normal`}
        rows="6"
        bind:value={draft.bodyMarkdown}></textarea>{/snippet}
  </AnnouncementField>
  <div class="grid gap-4 sm:grid-cols-2">
    <AnnouncementField label="链接动作">
      {#snippet children(id)}<select {id} class={inputClass()} bind:value={draft.actionType}>
          <option value="NONE">无</option><option value="INTERNAL">站内链接</option><option
            value="EXTERNAL">外部链接</option
          >
        </select>{/snippet}
    </AnnouncementField>
    {#if draft.actionType !== 'NONE'}
      <AnnouncementField
        label="链接文字"
        validated
        error={issue('actionLabel')}
        help="启用链接时必填。"
      >
        {#snippet children(id, error)}<input
            {id}
            class={inputClass(error)}
            bind:value={draft.actionLabel}
            required
            aria-invalid={error ? true : undefined}
            aria-describedby={`${id}-help${error ? ` ${id}-error` : ''}`}
            onblur={() => {
              touched.actionLabel = true;
            }}
          />{/snippet}
      </AnnouncementField>
    {/if}
  </div>
  {#if draft.actionType !== 'NONE'}
    <AnnouncementField
      label={draft.actionType === 'INTERNAL' ? '站内路径' : '外部地址'}
      validated
      error={issue('actionTarget')}
      help="站内路径以 / 开头；外部地址使用 HTTP 或 HTTPS。"
    >
      {#snippet children(id, error)}<input
          {id}
          class={inputClass(error)}
          type={draft.actionType === 'EXTERNAL' ? 'url' : 'text'}
          placeholder={draft.actionType === 'INTERNAL' ? '/about' : 'https://example.com/news'}
          bind:value={draft.actionTarget}
          required
          aria-invalid={error ? true : undefined}
          aria-describedby={`${id}-help${error ? ` ${id}-error` : ''}`}
          onblur={() => {
            touched.actionTarget = true;
          }}
        />{/snippet}
    </AnnouncementField>
  {/if}
  <div class="grid gap-4 sm:grid-cols-2">
    <AnnouncementField
      label="开始时间"
      validated
      error={issue('startsAt')}
      help="设置结束时间时必填；定时发布需选择未来时间。"
    >
      {#snippet children(id, error)}<input
          {id}
          class={inputClass(error)}
          type="datetime-local"
          step="1"
          bind:value={draft.startsAt}
          disabled={identityLocked}
          aria-invalid={error ? true : undefined}
          aria-describedby={`${id}-help${error ? ` ${id}-error` : ''}`}
          onblur={() => {
            touched.startsAt = true;
          }}
        />{/snippet}
    </AnnouncementField>
    <AnnouncementField
      label="结束时间"
      validated
      error={issue('endsAt')}
      help="选填，必须晚于开始时间。"
    >
      {#snippet children(id, error)}<input
          {id}
          class={inputClass(error)}
          type="datetime-local"
          step="1"
          bind:value={draft.endsAt}
          aria-invalid={error ? true : undefined}
          aria-describedby={`${id}-help${error ? ` ${id}-error` : ''}`}
          onblur={() => {
            touched.endsAt = true;
          }}
        />{/snippet}
    </AnnouncementField>
  </div>
  {#if timeZone}<p class="text-xs text-fg-muted">时区：{timeZone}</p>{/if}
</fieldset>
