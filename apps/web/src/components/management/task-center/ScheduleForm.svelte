<script lang="ts">
  import { untrack } from 'svelte';

  import type { RequestConfig, ScheduleInput } from '@/api/task-center/configuration.types';
  import { utf8Length } from '@/shared/text-length';

  import { buttonClass, fieldClass, inputClass, primaryClass } from './styles';
  let {
    initial,
    configs,
    busy,
    system = false,
    manual = false,
    onsave,
    oncancel,
  }: {
    initial: ScheduleInput;
    configs: readonly RequestConfig[];
    busy: boolean;
    system?: boolean;
    manual?: boolean;
    onsave: (value: ScheduleInput) => Promise<void>;
    oncancel: () => void;
  } = $props();
  let draft = $state(untrack(() => structuredClone(initial)));
  let sites = $state(untrack(() => initial.configuration.site_ids.join('\n')));
  let error = $state('');
  const choices = $derived(
    configs.filter(
      (c) =>
        c.purpose === draft.kind && c.enabled && !c.archived && (system || c.parameters.signed),
    ),
  );
  async function submit(event: SubmitEvent) {
    event.preventDefault();
    error = '';
    if (!manual && (!draft.name.trim() || utf8Length(draft.name) > 120)) {
      error = '名称不能为空，且不能超过 120 字节。';
      return;
    }
    const config = choices.find((c) => c.id === draft.configuration.request_config_id);
    if (!config) {
      error = '请选择已启用的请求配置。';
      return;
    }
    await onsave({
      ...draft,
      configuration: {
        ...draft.configuration,
        user_agent: config.parameters.user_agent,
        headers: config.parameters.headers,
        timeout_seconds: config.parameters.timeout_seconds,
        retries: config.parameters.retries,
        interval_ms: config.parameters.interval_ms,
        signed: config.parameters.signed,
        carrier: config.parameters.carrier,
        ttl_seconds: config.parameters.ttl_seconds,
        max_redirects: config.parameters.max_redirects,
        site_ids: sites.split(/[\s,]+/u).filter(Boolean),
      },
    });
  }
</script>

<form class="grid min-w-0 gap-6 border-y border-line py-6" onsubmit={submit}>
  <h2 class="text-xl font-semibold">
    {manual ? '新建手动任务' : initial.revision ? '编辑调度' : '新建调度'}
  </h2>
  <fieldset disabled={busy} class="grid min-w-0 gap-5 sm:grid-cols-2">
    {#if !manual}
      <label class={fieldClass}
        >名称<input class={inputClass} maxlength="120" required bind:value={draft.name} /></label
      >
    {/if}
    <label class={fieldClass}
      >任务类型<select class={inputClass} bind:value={draft.kind}>
        <option value="monitor">站点检测</option><option value="rss">RSS 抓取</option>
      </select></label
    >
    <label class={fieldClass}
      >请求配置<select
        class={inputClass}
        required
        bind:value={draft.configuration.request_config_id}
      >
        <option value="" disabled>请选择</option>
        {#each choices as config (config.id)}<option value={config.id}
            >{config.name}{config.parameters.signed ? '' : '（未签名）'}</option
          >{/each}
      </select></label
    >
    {#if !manual}
      <label class={fieldClass}
        >调度方式<select class={inputClass} bind:value={draft.configuration.mode}>
          <option value="interval">固定间隔</option><option value="cron">Cron</option>
        </select></label
      >
      {#if draft.configuration.mode === 'interval'}
        <label class={fieldClass}
          >执行间隔（秒）<input
            class={inputClass}
            type="number"
            min="60"
            max="2678400"
            required
            bind:value={draft.configuration.interval_seconds}
          /></label
        >
      {:else}
        <label class={fieldClass}
          >Cron（分 时 日 月 周）<input
            class={inputClass}
            required
            bind:value={draft.configuration.cron}
          /></label
        >
      {/if}
      <label class={fieldClass}
        >时区<input class={inputClass} required bind:value={draft.configuration.timezone} /></label
      >
    {/if}
    <label class={fieldClass}
      >目标范围<select class={inputClass} bind:value={draft.configuration.scope}>
        <option value="all">全部可见站点</option><option value="selected">指定站点</option>
      </select></label
    >
    {#if draft.configuration.scope === 'selected'}
      <label class={`${fieldClass} sm:col-span-2`}
        >站点 ID（每行一个）<textarea
          class={`${inputClass} py-3`}
          rows="3"
          required
          bind:value={sites}></textarea></label
      >
    {/if}
    {#if draft.kind === 'rss'}
      <label class={fieldClass}
        >订阅源范围<select class={inputClass} bind:value={draft.configuration.feed_mode}>
          <option value="default">默认订阅源</option><option value="all">全部启用订阅源</option>
        </select></label
      >
    {/if}
    <label class={fieldClass}
      >并发数<input
        class={inputClass}
        type="number"
        min="1"
        max="16"
        required
        bind:value={draft.configuration.concurrency}
      /></label
    >
    <label class={fieldClass}
      >运行时限（秒）<input
        class={inputClass}
        type="number"
        min="60"
        max="604800"
        required
        bind:value={draft.configuration.run_timeout_seconds}
      /></label
    >
    {#if !manual}
      <label class="flex min-h-11 items-center gap-3 text-sm"
        ><input type="checkbox" bind:checked={draft.enabled} />启用调度</label
      >
    {/if}
  </fieldset>
  {#if error}<p class="text-sm text-danger-fg" role="alert">{error}</p>{/if}
  <div class="flex flex-wrap justify-end gap-3">
    <button type="button" class={buttonClass} disabled={busy} onclick={oncancel}>取消</button>
    <button class={primaryClass} disabled={busy} aria-busy={busy}
      >{manual ? '创建任务' : '保存调度'}</button
    >
  </div>
</form>
