<script lang="ts">
  import { untrack } from 'svelte';

  import {
    applicableScopes,
    defaultExpiry,
    expirationPayload,
  } from '@/application/api-keys/api-keys.model';
  import {
    type ApiClientAudience,
    type ApiClientCreatePayload,
    apiClientScopeLabels,
    apiClientScopesByAudience,
    type ApiClientSummary,
    type ApiClientUpdatePayload,
    defaultApiClientScopes,
  } from '@/application/api-keys/api-keys.types';

  import { buttonClass, errorClass, inputClass, primaryClass } from './api-keys.styles';
  import ApiKeyExpiryFields from './ApiKeyExpiryFields.svelte';

  type ClientFormSubmission =
    | { readonly kind: 'create'; readonly payload: ApiClientCreatePayload }
    | { readonly kind: 'edit'; readonly payload: ApiClientUpdatePayload };
  interface Props {
    client?: ApiClientSummary;
    busy: boolean;
    onsubmit: (value: ClientFormSubmission) => Promise<void>;
    oncancel: () => void;
    ondirty: (dirty: boolean) => void;
  }
  let { client, busy, onsubmit, oncancel, ondirty }: Props = $props();
  const initial = untrack(() => ({
    name: client?.name ?? '',
    description: client?.description ?? '',
    audience: client?.audience ?? 'INTERNAL',
    scopes: [...(client?.scopes ?? defaultApiClientScopes('INTERNAL'))],
  }));
  let name = $state(initial.name);
  let description = $state(initial.description);
  let audience = $state<ApiClientAudience>(initial.audience);
  let scopes = $state([...initial.scopes]);
  let expiresAt = $state(defaultExpiry(initial.audience));
  let neverExpires = $state(false);
  let validationError = $state<string | null>(null);

  function changeAudience(event: Event): void {
    if (!(event.currentTarget instanceof HTMLSelectElement)) return;
    audience = event.currentTarget.value === 'EXTERNAL' ? 'EXTERNAL' : 'INTERNAL';
    scopes = applicableScopes(scopes, audience);
    expiresAt = defaultExpiry(audience);
    neverExpires = false;
    validationError = null;
  }

  async function submit(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    validationError = null;
    if (scopes.length === 0) {
      validationError = '请至少选择一项调用权限。';
      return;
    }
    if (!name.trim()) {
      validationError = '请输入调用方名称。';
      return;
    }
    const fields = { name: name.trim(), description: description.trim(), scopes: [...scopes] };
    if (client) {
      await onsubmit({
        kind: 'edit',
        payload: { ...fields, enabled: client.disabled_at === null },
      });
      return;
    }
    const date = new Date(expiresAt);
    const expiration = expirationPayload(
      audience,
      {
        expires_at: neverExpires || !Number.isFinite(date.getTime()) ? null : date.toISOString(),
        never_expires: neverExpires,
      },
      Date.now(),
    );
    if (expiration === null) {
      validationError =
        audience === 'INTERNAL' ? '到期时间必须在未来 90 天内。' : '请选择未来的到期时间。';
      return;
    }
    await onsubmit({ kind: 'create', payload: { ...fields, audience, ...expiration } });
  }
</script>

<form
  class="flex min-h-0 flex-1 flex-col"
  onsubmit={submit}
  oninput={() => ondirty(true)}
  onchange={() => ondirty(true)}
>
  <fieldset class="grid gap-5 p-6" disabled={busy}>
    <label class="grid gap-2 text-sm font-medium"
      >名称<input
        class={inputClass}
        name="name"
        maxlength="128"
        required
        bind:value={name}
      /></label
    >
    <label class="grid gap-2 text-sm font-medium"
      >调用类型
      <select
        class={inputClass}
        name="audience"
        value={audience}
        onchange={changeAudience}
        disabled={Boolean(client)}
      >
        <option value="INTERNAL">内部服务</option><option value="EXTERNAL">外部服务</option>
      </select>
      {#if client}<span class="text-xs font-normal text-fg-muted">调用类型在创建后不可更改。</span
        >{/if}
    </label>
    <label class="grid gap-2 text-sm font-medium"
      >用途说明<textarea
        class={`${inputClass} min-h-24 py-3`}
        name="description"
        maxlength="512"
        bind:value={description}></textarea></label
    >
    <fieldset
      class="grid gap-2"
      aria-describedby={validationError ? 'client-form-error' : undefined}
    >
      <legend class="mb-2 text-sm font-medium">调用权限</legend>
      {#each apiClientScopesByAudience[audience] as scope (scope)}
        <label
          class="flex min-h-11 items-center gap-3 rounded-md border border-line px-3 py-2 text-sm sm:min-h-10"
        >
          <input
            class="size-4 shrink-0 accent-primary"
            type="checkbox"
            name="scopes"
            value={scope}
            bind:group={scopes}
          />
          <span class="grid gap-1"
            ><span>{apiClientScopeLabels[scope]}</span><span class="font-mono text-xs text-fg-muted"
              >{scope}</span
            ></span
          >
        </label>
      {/each}
    </fieldset>
    {#if !client}<ApiKeyExpiryFields {audience} bind:expiresAt bind:neverExpires />{/if}
    {#if validationError}<p id="client-form-error" class={errorClass} role="alert">
        {validationError}
      </p>{/if}
  </fieldset>
  <footer
    class="sticky bottom-0 mt-auto flex shrink-0 justify-end gap-3 border-t border-line bg-surface px-6 py-4"
  >
    <button class={buttonClass} type="button" disabled={busy} onclick={oncancel}>取消</button>
    <button class={primaryClass} type="submit" disabled={busy} aria-busy={busy}
      >{busy ? '正在保存' : client ? '保存设置' : '创建并生成密钥'}</button
    >
  </footer>
</form>
