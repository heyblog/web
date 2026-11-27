<script lang="ts">
  import { onMount } from 'svelte';

  import type { ProofCheck, SigningKey } from '@/api/task-center/signing.types';
  import { signingAPI } from '@/api/task-center/task-center.browser';
  import { date, message, state as stateLabel } from '@/application/task-center/model';

  import { buttonClass, fieldClass, inputClass, panelClass, primaryClass } from './styles';
  let token = $state('');
  let observedMethod = $state('');
  let observedURL = $state('');
  let result = $state<ProofCheck | null>(null);
  let keys = $state<SigningKey[]>([]);
  let busy = $state(false);
  let error = $state('');
  let keyError = $state('');
  const labels = {
    valid: '签名有效',
    invalid: '签名无效',
    expired: '签名已过期',
    revoked: '签名密钥已撤销',
    missing: '未提供签名',
  };
  async function loadKeys() {
    keyError = '';
    try {
      keys = await signingAPI.publicKeys();
    } catch (error) {
      keyError = message(error);
    }
  }
  async function verify(event: SubmitEvent) {
    event.preventDefault();
    if (busy) return;
    busy = true;
    error = '';
    result = null;
    try {
      const value = token.trim();
      result = await signingAPI.verify(
        value.includes('HeyBlog-Proof/')
          ? (value.split('HeyBlog-Proof/')[1]?.split(/\s/u)[0] ?? '')
          : value,
        observedMethod && observedURL ? { method: observedMethod, url: observedURL } : undefined,
      );
    } catch (cause) {
      error = message(cause);
    } finally {
      busy = false;
    }
  }
  onMount(() => {
    void loadKeys();
  });
</script>

<div class="grid min-w-0 gap-12">
  <form class={panelClass} onsubmit={verify}>
    <label class={fieldClass}
      >请求签名或 User-Agent 日志<textarea
        class={`${inputClass} py-3 font-mono text-sm`}
        rows="6"
        maxlength="10000"
        spellcheck="false"
        autocomplete="off"
        bind:value={token}></textarea></label
    >
    <p class="text-sm text-fg-muted">
      可粘贴 X-HeyBlog-Proof 的值，或包含 HeyBlog-Proof 的 User-Agent。
    </p>
    <div class="grid gap-4 sm:grid-cols-2">
      <label class={fieldClass}
        >观察到的请求方法（可选）<select
          class={inputClass}
          required={Boolean(observedURL)}
          bind:value={observedMethod}
          ><option value="">只检查签名</option><option value="GET">GET</option><option value="HEAD"
            >HEAD</option
          ></select
        ></label
      >
      <label class={fieldClass}
        >观察到的完整 URL<input
          class={inputClass}
          type="url"
          required={Boolean(observedMethod)}
          bind:value={observedURL}
        /></label
      >
    </div>
    <button class={`${primaryClass} w-fit`} disabled={busy} aria-busy={busy}
      >{busy ? '正在验证…' : '验证签名'}</button
    >{#if error}<p class="text-sm text-danger-fg" role="alert">{error}</p>{/if}
  </form>
  {#if result}<section class={panelClass} aria-live="polite">
      <h2 class="text-xl font-semibold">
        {result.status === 'valid'
          ? result.binding === 'matched'
            ? '签名有效，请求参数一致'
            : result.binding === 'mismatch'
              ? '签名有效，请求参数不匹配'
              : '签名有效，尚未核对实际请求'
          : labels[result.status]}
      </h2>
      {#if result.claims}<dl class="grid gap-4 text-sm sm:grid-cols-2">
          <div>
            <dt class="text-fg-muted">请求方法</dt>
            <dd>{result.claims.method}</dd>
          </div>
          <div>
            <dt class="text-fg-muted">授权平台</dt>
            <dd>{result.claims.platform}</dd>
          </div>
          <div class="sm:col-span-2">
            <dt class="text-fg-muted">目标 URL</dt>
            <dd class="break-all">{result.claims.url}</dd>
          </div>
          <div>
            <dt class="text-fg-muted">签发时间</dt>
            <dd>{date(new Date(result.claims.iat * 1000).toISOString())}</dd>
          </div>
          <div>
            <dt class="text-fg-muted">到期时间</dt>
            <dd>{date(new Date(result.claims.exp * 1000).toISOString())}</dd>
          </div>
          <div class="sm:col-span-2">
            <dt class="text-fg-muted">请求编号</dt>
            <dd class="break-all">{result.claims.jti}</dd>
          </div>
        </dl>{/if}
      <p class="text-sm text-fg-muted">
        签名证明 HeyBlog 授权了这些请求参数。请与实际日志中的方法和 URL
        核对；平台字段不能独立证明请求节点的位置。
      </p>
    </section>{/if}
  <section class="grid gap-4">
    <div class="flex flex-wrap justify-between gap-3">
      <h2 class="text-xl font-semibold">已发布公钥</h2>
      <button class={buttonClass} onclick={() => loadKeys()}>刷新公钥</button>
    </div>
    {#if keyError}<p class="text-sm text-danger-fg" role="alert">
        {keyError}
      </p>{/if}{#if !keys.length && !keyError}<p class="text-sm text-fg-muted">
        暂无已发布公钥。
      </p>{/if}{#each keys as key (key.id)}<details class={panelClass}>
        <summary class="cursor-pointer text-sm break-all"
          >{key.platform} · {stateLabel(key.state)} · {key.id}</summary
        >
        <pre class="overflow-x-auto rounded-md bg-subtle p-4 text-xs">{JSON.stringify(
            key.public_key,
            null,
            2,
          )}</pre>
      </details>{/each}
  </section>
</div>
