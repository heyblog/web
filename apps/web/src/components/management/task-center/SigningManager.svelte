<script lang="ts">
  import { onMount } from 'svelte';

  import type { SigningKey } from '@/api/task-center/signing.types';
  import { date, state as stateLabel } from '@/application/task-center/model';
  import { SigningController } from '@/application/task-center/signing.controller.svelte';

  import SigningPolicy from './SigningPolicy.svelte';
  import { buttonClass, inputClass, panelClass, primaryClass } from './styles';
  const controller = new SigningController();
  let platform = $state<'CF' | 'EO'>('CF');
  let pending = $state<{ key: SigningKey; action: 'activate' | 'revoke' } | null>(null);
  const actionLabels: Readonly<Record<string, string>> = {
    generated: '生成密钥',
    publish: '发布公钥',
    activate: '启用密钥',
    revoke: '撤销密钥',
    policy_updated: '修改策略',
  };
  onMount(() => {
    void controller.refresh();
  });
</script>

<div class="grid min-w-0 gap-8">
  <div class="flex flex-wrap items-center justify-between gap-3">
    <a
      class="min-h-11 py-3 text-sm text-tint-fg underline-offset-4 hover:underline"
      href="/request-signing">公开验签页</a
    ><button class={buttonClass} disabled={controller.busy} onclick={() => controller.refresh()}
      >刷新</button
    >
  </div>
  {#if controller.error}<p class="rounded-md bg-danger-bg p-4 text-sm text-danger-fg" role="alert">
      {controller.error}
    </p>{/if}
  {#if controller.notice}<p class="text-sm text-success-fg" role="status">
      {controller.notice}
    </p>{/if}
  {#if controller.busy}<p class="text-sm text-fg-muted" role="status">正在处理…</p>{/if}
  {#if controller.policy}{#key controller.policy}<SigningPolicy
        policy={controller.policy}
        busy={controller.busy}
        onsave={(value) => controller.save(value)}
      />{/key}{/if}
  <section class="grid gap-5">
    <h2 class="text-xl font-semibold">签名密钥</h2>
    <div class="flex flex-wrap items-end gap-3">
      <label class="grid gap-2 text-sm font-medium"
        >平台<select class={inputClass} bind:value={platform}
          ><option value="CF">Cloudflare · 境外</option><option value="EO">EdgeOne · 大陆</option
          ></select
        ></label
      ><button
        class={primaryClass}
        disabled={controller.busy}
        onclick={() => controller.generate(platform)}>生成新密钥</button
      >
    </div>
    <p class="text-sm text-fg-muted">
      轮换时先生成并发布新公钥，再启用新密钥。旧公钥保留用于历史验签。
    </p>
    {#if pending}<div class={panelClass} role="alert">
        <p>
          {pending.action === 'revoke'
            ? '撤销后将立即停止使用此密钥，历史签名也会显示为已撤销。'
            : '启用后，此平台的新请求将使用新密钥，原活动密钥转为历史密钥。'}
        </p>
        <p class="text-xs break-all text-fg-muted">{pending.key.id}</p>
        <div class="flex gap-3">
          <button
            class={buttonClass}
            disabled={controller.busy}
            onclick={async () => {
              if (pending) await controller.transition(pending.key, pending.action);
              pending = null;
            }}>确认{pending.action === 'revoke' ? '撤销' : '启用'}</button
          ><button
            class={buttonClass}
            onclick={() => {
              pending = null;
            }}>取消</button
          >
        </div>
      </div>{/if}
    {#if controller.policy && !controller.keys.length}<p class="text-sm text-fg-muted">
        暂无密钥。请分别为两个平台生成并启用密钥。
      </p>{/if}
    {#each controller.keys as key (key.id)}<article class={panelClass}>
        <div class="flex flex-wrap justify-between gap-3">
          <div class="grid gap-2">
            <h3 class="font-semibold">{key.platform} · {stateLabel(key.state)}</h3>
            <p class="text-xs break-all text-fg-muted">{key.id}</p>
            <p class="text-sm text-fg-muted">生成于 {date(key.created_at)}</p>
          </div>
          <div class="flex flex-wrap items-start gap-2">
            {#if key.state === 'generated'}<button
                class={buttonClass}
                disabled={controller.busy}
                onclick={() => controller.transition(key, 'publish')}>发布公钥</button
              >{/if}{#if key.state === 'published'}<button
                class={buttonClass}
                disabled={controller.busy}
                onclick={() => {
                  pending = { key, action: 'activate' };
                }}>启用</button
              >{/if}{#if key.state !== 'revoked'}<button
                class={buttonClass}
                disabled={controller.busy}
                onclick={() => {
                  pending = { key, action: 'revoke' };
                }}>撤销</button
              >{/if}
          </div>
        </div>
        <details>
          <summary class="cursor-pointer text-sm">查看公钥（JWK）</summary>
          <pre class="mt-3 overflow-x-auto rounded-md bg-subtle p-4 text-xs">{JSON.stringify(
              key.public_key,
              null,
              2,
            )}</pre>
        </details>
      </article>{/each}
  </section>
  <section class="grid gap-4">
    <h2 class="text-xl font-semibold">操作记录</h2>
    {#if !controller.events.length}<p class="text-sm text-fg-muted">
        暂无操作记录。
      </p>{/if}{#each controller.events as event (event.id)}<article
        class="grid gap-2 border-b border-line py-4 text-sm"
      >
        <p>{actionLabels[event.action] ?? '密钥操作'} · {date(event.created_at)}</p>
        <p class="text-xs break-all text-fg-muted">
          操作人：{event.actor_id ?? '系统'}{event.key_id ? ` · 密钥：${event.key_id}` : ''}
        </p>
      </article>{/each}
  </section>
</div>
