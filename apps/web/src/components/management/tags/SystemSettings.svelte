<script lang="ts">
  import { untrack } from 'svelte';

  import type { AIModel, AISettings } from '@/api/taxonomy/system-settings.types';
  import { readModels, readSettings, saveSettings } from '@/api/taxonomy/taxonomy.browser';
  import { taxonomyMessage } from '@/application/taxonomy/taxonomy.messages';

  import { inputClass, outlineClass, primaryClass } from './tags.styles';

  interface Props {
    initialSettings: AISettings | null;
    initialError: string | null;
  }
  let { initialSettings, initialError }: Props = $props();
  let settings = $state<AISettings | null>(untrack(() => initialSettings));
  let modelID = $state(untrack(() => initialSettings?.model_id ?? ''));
  let models = $state<readonly AIModel[]>([]);
  let error = $state(untrack(() => initialError ?? ''));
  let message = $state('');
  let pending = $state(false);
  let loadedModels = $state(false);
  let selectedAvailable = $derived(
    models.some((model) => model.id === modelID && model.status === 'available'),
  );

  async function refresh(): Promise<void> {
    if (pending) return;
    pending = true;
    error = '';
    message = '';
    const [settingsResult, modelsResult] = await Promise.all([readSettings(), readModels()]);
    pending = false;
    if (settingsResult.ok) {
      settings = settingsResult.value;
      modelID = settingsResult.value.model_id;
    } else error = taxonomyMessage(settingsResult.code);
    if (modelsResult.ok) {
      models = modelsResult.value.models;
      loadedModels = true;
    } else {
      models = [];
      loadedModels = false;
      error = taxonomyMessage(modelsResult.code);
    }
  }

  async function save(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    if (pending || !settings || !selectedAvailable) return;
    pending = true;
    error = '';
    message = '';
    const result = await saveSettings(modelID, settings.revision);
    pending = false;
    if (result.ok) {
      settings = result.value;
      message = '已保存，后续生成将使用此模型。';
    } else error = taxonomyMessage(result.code);
  }
</script>

<section
  class="grid max-w-2xl gap-6 rounded-md border border-line bg-surface p-6"
  aria-busy={pending}
>
  <header>
    <h2 class="text-lg font-semibold">标签 slug 生成</h2>
    <p class="mt-2 text-sm text-fg-muted">所有标签共用此模型。只有系统管理员可以修改。</p>
  </header>
  {#if settings}
    <dl class="grid gap-3 text-sm">
      <div class="flex flex-wrap justify-between gap-3">
        <dt class="text-fg-muted">服务状态</dt>
        <dd>{settings.configured ? '已配置' : '未配置'}</dd>
      </div>
      <div class="flex flex-wrap justify-between gap-3">
        <dt class="text-fg-muted">当前模型</dt>
        <dd class="wrap-anywhere">
          {settings.model_id === 'deepseek/deepseek-flash'
            ? 'DeepSeek-V4.1-Flash'
            : settings.model_id}
        </dd>
      </div>
    </dl>
    {#if !settings.configured}<p class="rounded-md bg-warning-bg p-3 text-sm text-warning-fg">
        生成服务尚未配置，标签 slug 可以手动填写。
      </p>{/if}
  {/if}
  <form class="grid gap-4 border-t border-line pt-5" onsubmit={save}>
    <label class="grid gap-1.5 text-sm font-medium"
      >生成模型<select class={inputClass} bind:value={modelID} disabled={pending || !loadedModels}>
        {#if !models.some((model) => model.id === modelID)}<option value={modelID}
            >{modelID || '请加载可用模型'}</option
          >{/if}
        {#each models as model (model.id)}<option
            value={model.id}
            disabled={model.status !== 'available'}
            >{model.name} {model.status !== 'available' ? '（不可用）' : ''}</option
          >{/each}
      </select></label
    >
    <div class="flex flex-wrap gap-3">
      <button class={outlineClass} type="button" disabled={pending} onclick={refresh}
        >{pending ? '正在加载…' : loadedModels ? '刷新设置和模型' : '加载可用模型'}</button
      >
      <button
        class={primaryClass}
        type="submit"
        disabled={pending ||
          !settings?.configured ||
          !selectedAvailable ||
          modelID === settings?.model_id}>保存设置</button
      >
    </div>
  </form>
  {#if message}<p class="text-sm text-success-fg" role="status">{message}</p>{/if}
  {#if error}<p class="rounded-md bg-danger-bg p-3 text-sm text-danger-fg" role="alert">
      {error}
    </p>{/if}
</section>
