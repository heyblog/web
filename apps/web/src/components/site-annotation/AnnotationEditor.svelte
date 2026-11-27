<script lang="ts">
  import { onMount, untrack } from 'svelte';

  import { requestAnnotation } from '@/api/site-annotation/annotation.browser';
  import {
    type AnnotationDetail,
    type AnnotationOptions,
    blogStatuses,
  } from '@/api/site-annotation/annotation.types';
  import { protectFormDraft } from '@/application/navigation/form-navigation.browser';
  import {
    type AnnotationDraft,
    annotationDraft,
    annotationPatch,
  } from '@/application/site-annotation/annotation-draft';
  import {
    annotationFailure,
    annotationTime,
    scopeLabels,
    statusLabels,
  } from '@/application/site-annotation/annotation-view';

  import {
    buttonClass,
    fieldClass,
    inputClass,
    linkClass,
    primaryClass,
    sectionClass,
    warningClass,
  } from './annotation.styles';
  import AnnotationLinks from './AnnotationLinks.svelte';
  import AnnotationLocations from './AnnotationLocations.svelte';
  import AnnotationTaxonomy from './AnnotationTaxonomy.svelte';
  import AnnotationValues from './AnnotationValues.svelte';

  let {
    initial,
    options,
    canReview,
  }: {
    initial: AnnotationDetail;
    options: AnnotationOptions | null;
    canReview: boolean;
  } = $props();

  let detail = $state.raw(untrack(() => initial));
  let draft = $state<AnnotationDraft>(annotationDraft(untrack(() => initial)));
  let reason = $state('');
  let message = $state('');
  let error = $state('');
  let conflict = $state(false);
  let latest = $state.raw<AnnotationDetail | null>(null);
  let saving = $state(false);
  let loading = $state(false);
  let ready = $state(false);
  let nextKey = -1;

  const patch = $derived(annotationPatch(draft, detail, reason));

  onMount(() => {
    ready = true;
    return protectFormDraft(() => ({
      dirty: patch !== null || Boolean(reason.trim()),
      busy: saving,
    }));
  });

  function allocateKey() {
    return nextKey--;
  }

  function replaceDraft(value: AnnotationDetail) {
    detail = value;
    draft = annotationDraft(value);
    reason = '';
    conflict = false;
    latest = null;
    error = '';
  }

  async function save(event: SubmitEvent) {
    event.preventDefault();
    if (!ready || !patch || saving || loading || conflict) return;
    saving = true;
    latest = null;
    message = '';
    error = '';
    try {
      const result = await requestAnnotation(detail.short_id, patch);
      if (result.kind === 'ready') {
        replaceDraft(result.detail);
        message = '标注已保存。';
      } else if (result.code === 'site_address_conflict') {
        error = '该网址已被其他站点使用。';
      } else if (result.status === 409) {
        conflict = true;
      } else {
        error = annotationFailure(result.status, 'save');
      }
    } finally {
      saving = false;
    }
  }

  async function loadLatest() {
    if (loading || saving) return;
    loading = true;
    message = '';
    error = '';
    try {
      const result = await requestAnnotation(detail.short_id);
      if (result.kind === 'ready') latest = result.detail;
      else error = annotationFailure(result.status, 'load');
    } finally {
      loading = false;
    }
  }
</script>

<div class="grid min-w-0 gap-6">
  {#if detail.pending_audit_id}
    <div class={warningClass}>
      <p>此站点有待审核的申请。</p>
      {#if canReview}
        <a class={linkClass} href={`/management/site-submissions/${detail.pending_audit_id}`}
          >查看待审核申请</a
        >
      {/if}
    </div>
  {/if}

  <form class="grid min-w-0 gap-6" method="post" autocomplete="off" onsubmit={save}>
    <fieldset class="grid min-w-0 gap-6" disabled={saving}>
      <section class={sectionClass} aria-labelledby="annotation-basics">
        <h2 id="annotation-basics" class="text-lg font-semibold">基本信息</h2>
        <div class="grid gap-4 sm:grid-cols-2">
          <label class={fieldClass}
            >名称<input
              class={inputClass}
              bind:value={draft.name}
              maxlength="160"
              required
            /></label
          >
          <label class={fieldClass}
            >网址<input
              class={inputClass}
              type="url"
              bind:value={draft.url}
              maxlength="2048"
              required
            /></label
          >
        </div>
        <label class={fieldClass}
          >简介<textarea
            class={`${inputClass} min-h-28 py-3`}
            rows="4"
            maxlength="2000"
            bind:value={draft.summary}></textarea></label
        >
        <div class="grid gap-4 sm:grid-cols-2">
          <label class={fieldClass}
            >标注状态<select class={inputClass} bind:value={draft.blog_status}>
              {#each blogStatuses as status (status)}<option value={status}
                  >{statusLabels[status]}</option
                >{/each}
            </select></label
          >
          <div class={fieldClass}>
            <span>检测访问范围</span>
            <p>
              {detail.access_scope === null ? '暂无检测结论' : scopeLabels[detail.access_scope]}
            </p>
          </div>
        </div>
      </section>

      <AnnotationTaxonomy bind:draft {options} {allocateKey} />
      <AnnotationLocations bind:draft {allocateKey} />
      <AnnotationLinks bind:draft incoming={detail.incoming} {allocateKey} />

      <section class={sectionClass} aria-labelledby="annotation-save">
        <h2 id="annotation-save" class="text-lg font-semibold">保存标注</h2>
        <label class={fieldClass}
          >变更原因（选填）<input class={inputClass} bind:value={reason} maxlength="2000" /></label
        >
        <div class="flex flex-wrap items-center justify-between gap-3">
          <p class="text-sm text-fg-muted">
            修订号 {detail.revision}{patch ? ' · 有未保存的改动' : ''}
          </p>
          <button
            class={primaryClass}
            type="submit"
            disabled={!ready || !patch || loading || conflict}
          >
            {saving ? '保存中…' : '保存标注'}
          </button>
        </div>
      </section>
    </fieldset>
  </form>

  {#if message}<p
      class="rounded-md border border-success bg-success-bg p-3 text-sm text-success-fg"
      role="status"
    >
      {message}
    </p>{/if}
  {#if error}<p
      class="rounded-md border border-danger bg-danger-bg p-3 text-sm text-danger-fg"
      role="alert"
    >
      {error}
    </p>{/if}
  {#if conflict}
    <div class={warningClass}>
      <p role="alert">站点已更新。你的改动仍在表单中。请查看最新保存值，再决定是否载入。</p>
      <div class="flex flex-wrap gap-3">
        <button class={buttonClass} type="button" disabled={loading} onclick={loadLatest}>
          {loading ? '正在加载…' : '查看最新保存值'}
        </button>
      </div>
    </div>
  {:else if error && patch && !latest}
    <div>
      <button class={buttonClass} type="button" disabled={loading || saving} onclick={loadLatest}>
        {loading ? '正在加载…' : '查看已保存值'}
      </button>
    </div>
  {/if}
  {#if latest}
    <section class={sectionClass} aria-labelledby="annotation-latest">
      <h2 id="annotation-latest" class="text-lg font-semibold">最新保存值</h2>
      <p class="text-sm text-fg-muted">
        修订号 {latest.revision} · {annotationTime(latest.updated_at)}
      </p>
      <AnnotationValues values={{ ...latest }} />
      <div class="flex flex-wrap gap-3">
        <button
          class={buttonClass}
          type="button"
          disabled={loading || saving}
          onclick={() => {
            if (latest) replaceDraft(latest);
          }}
        >
          放弃当前改动并载入最新值
        </button>
        <button
          class={buttonClass}
          type="button"
          onclick={() => {
            latest = null;
          }}>关闭已保存值</button
        >
      </div>
    </section>
  {/if}
</div>
