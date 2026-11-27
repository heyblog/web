<script lang="ts">
  import { untrack } from 'svelte';

  import type { Policy } from '@/api/task-center/signing.types';

  import { buttonClass, fieldClass, inputClass, panelClass } from './styles';
  let { policy, busy, onsave }: { policy: Policy; busy: boolean; onsave: (value: Policy) => void } =
    $props();
  let draft = $state(untrack(() => ({ ...policy })));
  let confirm = $state(false);
</script>

<form
  class={panelClass}
  onsubmit={(event) => {
    event.preventDefault();
    if (!draft.enabled && !confirm) {
      confirm = true;
      return;
    }
    onsave(draft);
    confirm = false;
  }}
>
  <h2 class="text-xl font-semibold">签名策略</h2>
  <fieldset class="grid gap-5 sm:grid-cols-2" disabled={busy}>
    <label class="flex min-h-11 items-center gap-3 text-sm sm:col-span-2"
      ><input
        type="checkbox"
        bind:checked={draft.enabled}
        onchange={() => {
          confirm = false;
        }}
      />允许签发请求签名</label
    >
    <label class={fieldClass}
      >签名载体<select class={inputClass} bind:value={draft.carrier}
        ><option value="both">User-Agent 与请求头</option><option value="header"
          >仅 X-HeyBlog-Proof 请求头</option
        ></select
      ></label
    >
    <label class={fieldClass}
      >签名有效期（秒）<input
        class={inputClass}
        type="number"
        min="60"
        max="300"
        required
        bind:value={draft.ttl_seconds}
      /></label
    >
  </fieldset>
  {#if draft.carrier === 'header'}<p class="text-sm text-warning-fg">
      仅请求头模式下，User-Agent 日志无法单独验签。
    </p>{/if}
  {#if confirm}<p class="rounded-md bg-warning-bg p-4 text-sm text-warning-fg" role="alert">
      关闭后，需要签名的请求将停止发送。确认暂停签发？
    </p>{/if}
  <button class={`${buttonClass} w-fit`} disabled={busy}
    >{confirm ? '确认关闭并保存' : '保存策略'}</button
  >
</form>
