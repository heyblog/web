<script lang="ts">
  import { untrack } from 'svelte';

  import type { RequestConfigInput } from '@/api/task-center/configuration.types';
  import { utf8Length } from '@/shared/text-length';

  import { buttonClass, fieldClass, inputClass, primaryClass } from './styles';
  let {
    initial,
    busy,
    system,
    onsave,
    oncancel,
  }: {
    initial: RequestConfigInput;
    busy: boolean;
    system: boolean;
    onsave: (value: RequestConfigInput) => Promise<void>;
    oncancel: () => void;
  } = $props();
  let draft = $state(untrack(() => structuredClone(initial)));
  let headers = $state(
    untrack(() =>
      Object.entries(initial.parameters.headers)
        .map(([key, value]) => `${key}: ${value}`)
        .join('\n'),
    ),
  );
  let error = $state('');
  async function submit(event: SubmitEvent) {
    event.preventDefault();
    error = '';
    if (!draft.name.trim() || utf8Length(draft.name) > 120) {
      error = '名称不能为空，且不能超过 120 字节。';
      return;
    }
    if (!draft.parameters.user_agent || utf8Length(draft.parameters.user_agent) > 256) {
      error = 'User-Agent 不能为空，且不能超过 256 字节。';
      return;
    }
    const parsed: Record<string, string> = {};
    for (const line of headers.split('\n').filter((line) => line.trim())) {
      const split = line.indexOf(':');
      const name = line.slice(0, split).trim();
      if (
        split < 1 ||
        !line.slice(split + 1).trim() ||
        Object.keys(parsed).some((key) => key.toLowerCase() === name.toLowerCase())
      ) {
        error = '请求头须按每行「名称: 值」填写，名称不能重复。';
        return;
      }
      parsed[name] = line.slice(split + 1).trim();
    }
    if (Object.keys(parsed).length > 16) {
      error = '请求头最多 16 项。';
      return;
    }
    await onsave({ ...draft, parameters: { ...draft.parameters, headers: parsed } });
  }
</script>

<form class="grid min-w-0 gap-6 border-y border-line py-6" onsubmit={submit}>
  <h2 class="text-xl font-semibold">{initial.revision ? '编辑请求配置' : '新建请求配置'}</h2>
  <fieldset class="grid min-w-0 gap-5 sm:grid-cols-2" disabled={busy}>
    <label class={fieldClass}
      >名称<input class={inputClass} required maxlength="120" bind:value={draft.name} /></label
    >
    <label class={fieldClass}
      >用途<select class={inputClass} bind:value={draft.purpose}>
        <option value="monitor">站点检测</option><option value="rss">RSS 抓取</option><option
          value="verification">HTTP 认证</option
        >
      </select></label
    >
    <label class={`${fieldClass} sm:col-span-2`}
      >User-Agent<input
        class={inputClass}
        required
        maxlength="256"
        bind:value={draft.parameters.user_agent}
      /></label
    >
    <label class={`${fieldClass} sm:col-span-2`}
      >请求头（每行「名称: 值」）<textarea
        class={`${inputClass} py-3`}
        rows="3"
        bind:value={headers}></textarea></label
    >
    <label class={fieldClass}
      >网络超时（秒）<input
        class={inputClass}
        type="number"
        min="1"
        max="30"
        required
        bind:value={draft.parameters.timeout_seconds}
      /></label
    >
    <label class={fieldClass}
      >请求间隔（毫秒）<input
        class={inputClass}
        type="number"
        min="0"
        max="60000"
        required
        bind:value={draft.parameters.interval_ms}
      /></label
    >
    <label class={fieldClass}
      >自动重试<select class={inputClass} bind:value={draft.parameters.retries}>
        <option value={0}>关闭</option><option value={1}>1 次</option><option value={2}>2 次</option
        >
      </select></label
    >
    <label class={fieldClass}
      >最大重定向<input
        class={inputClass}
        type="number"
        min="0"
        max="3"
        required
        bind:value={draft.parameters.max_redirects}
      /></label
    >
    <label class="flex min-h-11 items-center gap-3 text-sm"
      ><input
        type="checkbox"
        disabled={!system}
        bind:checked={draft.parameters.signed}
      />请求签名</label
    >
    <label class={fieldClass}
      >签名载体<select class={inputClass} disabled={!system} bind:value={draft.parameters.carrier}>
        <option value="both">UA＋Header</option><option value="header">仅 Header</option>
      </select></label
    >
    <label class={fieldClass}
      >签名有效期（秒）<input
        class={inputClass}
        type="number"
        min="60"
        max="300"
        required
        disabled={!system}
        bind:value={draft.parameters.ttl_seconds}
      /></label
    >
    <label class="flex min-h-11 items-center gap-3 text-sm"
      ><input type="checkbox" bind:checked={draft.enabled} />启用配置</label
    >
  </fieldset>
  {#if error}<p class="text-sm text-danger-fg" role="alert">{error}</p>{/if}
  <div class="flex justify-end gap-3">
    <button type="button" class={buttonClass} disabled={busy} onclick={oncancel}>取消</button>
    <button class={primaryClass} disabled={busy}>保存配置</button>
  </div>
</form>
