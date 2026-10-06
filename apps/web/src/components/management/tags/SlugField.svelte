<script lang="ts">
  import { generateSlug } from '@/api/taxonomy/taxonomy.browser';
  import { taxonomyMessage } from '@/application/taxonomy/taxonomy.messages';

  interface Props {
    value?: string;
    name: string;
    description?: string;
    parentName?: string;
    tagID?: string;
    disabled?: boolean;
    required?: boolean;
    onbusy?: (busy: boolean) => void;
  }
  let {
    value = $bindable(''),
    name,
    description = '',
    parentName = '',
    tagID = '',
    disabled = false,
    required = true,
    onbusy,
  }: Props = $props();
  let pending = $state(false);
  let message = $state('');
  let error = $state('');

  async function generate(): Promise<void> {
    if (pending || disabled) return;
    pending = true;
    onbusy?.(true);
    message = '';
    error = '';
    const input = { name, description, parent_name: parentName, tag_id: tagID };
    const previousValue = value;
    try {
      const result = await generateSlug(input);
      if (
        name !== input.name ||
        description !== input.description ||
        parentName !== input.parent_name ||
        value !== previousValue
      ) {
        message = '内容已变化，请重新生成。';
      } else if (result.ok) {
        if (result.value.state === 'needs_confirmation') {
          error = `候选“${result.value.slug}”与${result.value.conflicts.map((tag) => `“${tag.name}”`).join('、') || '其他标签'}冲突。相同含义请在标签管理中添加同义名称或合并标签；不同含义请填写独立 slug。`;
          return;
        }
        value = result.value.slug;
        message = '已生成，请确认后保存。';
      } else error = taxonomyMessage(result.code);
    } finally {
      pending = false;
      onbusy?.(false);
    }
  }
</script>

<div class="grid min-w-0 gap-2" aria-busy={pending}>
  <label class="grid min-w-0 gap-1.5 text-sm font-medium">
    Slug
    <input
      class="min-h-11 min-w-0 rounded-md border border-line-strong bg-surface px-3 sm:min-h-10"
      bind:value
      {required}
      maxlength="160"
      pattern="[a-z0-9]+(?:-[a-z0-9]+)*"
      disabled={disabled || pending}
      placeholder="例如：independent-writing"
    />
  </label>
  <div class="flex flex-wrap items-center gap-2">
    <button
      class="min-h-11 rounded-md border border-line-strong px-3 text-sm font-medium hover:bg-subtle disabled:opacity-50 sm:min-h-10"
      type="button"
      disabled={disabled || pending || !name.trim()}
      onclick={generate}>{pending ? '正在生成…' : '生成 slug'}</button
    >
    <span class="text-xs text-fg-muted">小写英文、数字和连字符</span>
  </div>
  {#if message}<p class="text-sm text-fg-muted" role="status">{message}</p>{/if}
  {#if error}<p class="text-sm text-danger-fg" role="alert">{error}</p>{/if}
</div>
