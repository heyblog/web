<script lang="ts">
  import { IconBrandGithub } from '@tabler/icons-svelte';
  import { onDestroy } from 'svelte';

  import type {
    OAuthCompletion,
    OAuthIntent,
    OAuthProvider,
    OAuthRequest,
  } from '@/api/auth/oauth.types';
  import { authErrorMessage } from '@/application/auth/auth-page';
  import { safeNext } from '@/application/auth/auth-routing';
  import { createOAuthPopup } from '@/application/auth/oauth-popup.browser';
  import blogsclubIcon from '@/assets/brands/blogsclub.png?url';

  let {
    provider,
    intent = 'login',
    next = '/dashboard',
    claimId,
    label,
    disabled = false,
    prepare,
    oncomplete,
  }: {
    provider: OAuthProvider;
    intent?: OAuthIntent;
    next?: string;
    claimId?: string;
    label?: string;
    disabled?: boolean;
    prepare?: () => Promise<OAuthRequest | null>;
    oncomplete?: (result: OAuthCompletion) => void;
  } = $props();
  let busy = $state(false);
  let error = $state('');
  let fallback = $state<string>();
  const name = $derived(provider === 'github' ? 'GitHub' : 'BlogsClub');
  const popup = createOAuthPopup({
    busy: (value) => {
      busy = value;
    },
    error: (code, location) => {
      error = authErrorMessage(code) ?? '';
      fallback = location;
    },
    complete(result) {
      error = '';
      fallback = undefined;
      if (oncomplete) oncomplete(result);
      else window.location.assign(safeNext(result.next));
    },
  });
  export function authorize() {
    if (disabled || busy) return;
    error = '';
    fallback = undefined;
    void popup.launch(
      prepare ?? { provider, intent, next: safeNext(next), ...(claimId ? { claimId } : {}) },
    );
  }
  onDestroy(() => popup.dispose());
</script>

<div class="grid gap-3">
  <button
    class="inline-flex min-h-11 w-full items-center justify-center gap-2 rounded-sm px-4 text-sm font-semibold transition-colors outline-none focus-visible:ring-2 focus-visible:ring-focus focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 {provider ===
    'blogsclub'
      ? 'bg-blogsclub text-blogsclub-fg hover:bg-blogsclub-hover'
      : 'border border-line-strong bg-surface text-fg hover:bg-subtle'}"
    type="button"
    disabled={disabled || busy}
    aria-busy={busy}
    onclick={authorize}
  >
    {#if provider === 'github'}<IconBrandGithub size={18} stroke={1.8} aria-hidden="true" />
    {:else}
      <span
        class="inline-flex size-6 shrink-0 items-center justify-center rounded-sm bg-blogsclub-fg"
        aria-hidden="true"
      >
        <img src={blogsclubIcon} alt="" width="18" height="18" class="size-4.5 object-contain" />
      </span>
    {/if}
    {busy ? '正在授权…' : (label ?? `使用 ${name} 登录`)}
  </button>
  {#if error}<p class="text-sm text-danger-fg" role="alert">{error}</p>{/if}
  {#if fallback}<a
      class="min-h-11 text-sm text-tint-fg underline"
      href={fallback}
      data-astro-prefetch="false">在当前页面继续授权</a
    >{/if}
  {#if !prepare}<noscript
      ><a
        class="min-h-11 text-sm text-tint-fg underline"
        href={`/api/web/auth/${provider}/start?${new URLSearchParams({ intent, next: safeNext(next), mode: 'redirect', ...(claimId ? { claim_id: claimId } : {}) })}`}
        data-astro-prefetch="false">使用 {name} 授权</a
      ></noscript
    >{/if}
</div>
