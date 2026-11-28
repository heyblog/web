<script lang="ts">
  import { onMount, untrack } from 'svelte';

  import {
    requestAnnotation,
    requestNextAnnotation,
  } from '@/api/site-annotation/annotation.browser';
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
    isPreviewWindowOpen,
    openPreviewWindow,
    readPreviewLinkedPreference,
    writePreviewLinkedPreference,
  } from '@/application/site-annotation/annotation-preview.browser';
  import {
    annotationDetailPath,
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
  let navigatingNext = $state(false);
  let ready = $state(false);
  let previewLinked = $state(false);
  let previewNotice = $state('');
  let prefetched = $state.raw<AnnotationDetail | null>(null);
  let prefetchStatus = $state<'idle' | 'loading' | 'ready' | 'empty' | 'error'>('idle');
  let prefetchToken = 0;
  let nextKey = -1;

  const patch = $derived(annotationPatch(draft, detail, reason));
  const linkPageUrl = $derived(
    draft.resources.find((resource) => resource.kind === 'LINK_PAGE')?.url ?? '',
  );
  const siteHref = $derived(externalHref(draft.url));
  const linkPageHref = $derived(externalHref(linkPageUrl));

  function externalHref(value: string): string | null {
    const trimmed = value.trim();
    return /^https?:\/\//i.test(trimmed) ? trimmed : null;
  }

  function updateLinkPage(value: string) {
    const trimmed = value.trim();
    const existing = draft.resources.find((resource) => resource.kind === 'LINK_PAGE');
    if (!trimmed) {
      if (existing) {
        draft.resources = draft.resources.filter((resource) => resource.kind !== 'LINK_PAGE');
      }
      return;
    }
    if (existing) {
      existing.url = value;
    } else {
      draft.resources.push({ key: allocateKey(), kind: 'LINK_PAGE', url: value });
    }
  }

  function syncHistoryLink(shortId: string) {
    const historyLink = document.querySelector<HTMLAnchorElement>('[data-annotation-history-link]');
    if (historyLink) historyLink.href = `${annotationDetailPath(shortId)}/history`;
  }

  function openInPreview(url: string | null, options?: { focus?: boolean; enableLink?: boolean }) {
    if (!url) return;
    const opened = openPreviewWindow(url, { focus: options?.focus });
    if (!opened) {
      previewNotice = '浏览器拦截了右侧预览窗，请允许本站弹窗后重试。';
      return;
    }
    previewNotice = '';
    if (options?.enableLink !== false) {
      previewLinked = true;
      writePreviewLinkedPreference(true);
    }
  }

  async function prefetchNext(excludeShortId: string) {
    const token = ++prefetchToken;
    prefetched = null;
    prefetchStatus = 'loading';
    const nextResult = await requestNextAnnotation(excludeShortId);
    if (token !== prefetchToken) return;
    if (nextResult.kind === 'empty') {
      prefetchStatus = 'empty';
      return;
    }
    if (nextResult.kind !== 'ready') {
      prefetchStatus = 'error';
      return;
    }
    const detailResult = await requestAnnotation(nextResult.shortId);
    if (token !== prefetchToken) return;
    if (detailResult.kind === 'ready') {
      prefetched = detailResult.detail;
      prefetchStatus = 'ready';
    } else {
      prefetchStatus = 'error';
    }
  }

  function switchToDetail(nextDetail: AnnotationDetail, options?: { pushHistory?: boolean }) {
    replaceDraft(nextDetail);
    message = '';
    previewNotice = '';
    syncHistoryLink(nextDetail.short_id);
    if (options?.pushHistory !== false) {
      window.history.pushState(
        { shortId: nextDetail.short_id },
        '',
        annotationDetailPath(nextDetail.short_id),
      );
    }
    if (previewLinked || isPreviewWindowOpen()) {
      const targetUrl = externalHref(nextDetail.url);
      if (targetUrl) openInPreview(targetUrl, { focus: false, enableLink: true });
    }
    window.scrollTo({ top: 0, behavior: 'auto' });
    void prefetchNext(nextDetail.short_id);
  }

  onMount(() => {
    ready = true;
    previewLinked = readPreviewLinkedPreference() || isPreviewWindowOpen();
    syncHistoryLink(detail.short_id);
    window.history.replaceState({ shortId: detail.short_id }, '', window.location.href);
    void prefetchNext(detail.short_id);

    const onPopState = () => {
      const match = /^\/annotation\/([^/]+)\/?$/u.exec(window.location.pathname);
      const targetId = match?.[1] ? decodeURIComponent(match[1]) : '';
      if (!targetId || targetId === detail.short_id) return;
      loading = true;
      void requestAnnotation(targetId).then((result) => {
        loading = false;
        if (result.kind === 'ready') {
          switchToDetail(result.detail, { pushHistory: false });
        } else {
          window.location.reload();
        }
      });
    };
    window.addEventListener('popstate', onPopState);
    const cleanupGuard = protectFormDraft(() => ({
      dirty: patch !== null || Boolean(reason.trim()),
      busy: saving,
    }));
    return () => {
      window.removeEventListener('popstate', onPopState);
      cleanupGuard();
    };
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

  async function goNext(options?: { skipDirtyCheck?: boolean }) {
    if (navigatingNext || saving || loading) return;
    if (!options?.skipDirtyCheck && (patch !== null || Boolean(reason.trim()))) {
      if (!window.confirm('当前页面有未保存的改动，确定跳过并切换到下一个博客吗？')) return;
    }

    // 预加载已就绪时同步瞬切，并在同一点击手势内联动右侧预览窗
    if (prefetched && prefetched.short_id !== detail.short_id) {
      const nextDetail = prefetched;
      prefetched = null;
      prefetchStatus = 'idle';
      switchToDetail(nextDetail);
      return;
    }

    navigatingNext = true;
    message = '';
    error = '';
    try {
      const result = await requestNextAnnotation(detail.short_id);
      if (result.kind === 'ready') {
        const detailResult = await requestAnnotation(result.shortId);
        if (detailResult.kind === 'ready') {
          switchToDetail(detailResult.detail);
        } else {
          window.location.assign(annotationDetailPath(result.shortId));
        }
      } else if (result.kind === 'empty') {
        prefetchStatus = 'empty';
        message = '没有更多待标注的博客了。';
      } else {
        error = annotationFailure(result.status, 'load');
      }
    } finally {
      navigatingNext = false;
    }
  }

  async function save(event: SubmitEvent) {
    event.preventDefault();
    const submitter = event.submitter;
    const andNext = submitter instanceof HTMLButtonElement && submitter.value === 'save-and-next';
    if (!ready || !patch || saving || loading || navigatingNext || conflict) return;
    saving = true;
    latest = null;
    message = '';
    error = '';
    let saved = false;
    try {
      const result = await requestAnnotation(detail.short_id, patch);
      if (result.kind === 'ready') {
        replaceDraft(result.detail);
        message = '标注已保存。';
        saved = true;
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
    if (saved && andNext) {
      await goNext({ skipDirtyCheck: true });
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
      <section class={sectionClass} aria-labelledby="annotation-core">
        <div
          class="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 border-b border-line pb-3"
        >
          <div class="min-w-0">
            <h2 id="annotation-core" class="text-lg font-semibold wrap-anywhere">
              {draft.name || detail.short_id}
            </h2>
            <p class="text-sm wrap-anywhere text-fg-muted">{draft.url}</p>
          </div>
          <div class="flex flex-wrap items-center gap-3">
            {#if siteHref}
              <button
                class={buttonClass}
                type="button"
                disabled={!ready}
                onclick={() => openInPreview(siteHref, { focus: true, enableLink: true })}
              >
                {previewLinked ? '右窗联动中 · 打开站点' : '右侧预览窗 ↗'}
              </button>
              <a class={linkClass} href={siteHref} target="_blank" rel="noopener noreferrer"
                >新标签页 ↗</a
              >
            {/if}
            <button
              class={buttonClass}
              type="button"
              disabled={!ready || saving || loading || navigatingNext}
              onclick={() => goNext()}
            >
              {navigatingNext ? '正在抽取…' : '下一个博客 →'}
            </button>
          </div>
        </div>
        {#if previewNotice}
          <p class="text-sm text-warning-fg" role="status">{previewNotice}</p>
        {/if}

        <fieldset class="grid min-w-0 gap-2">
          <legend class="text-sm font-medium">是否为博客</legend>
          <div class="grid grid-cols-2 gap-2 sm:grid-cols-4">
            {#each blogStatuses as status (status)}
              {@const selected = draft.blog_status === status}
              <label
                class:border-primary={selected}
                class:bg-subtle={selected}
                class:text-tint-fg={selected}
                class:border-line-strong={!selected}
                class:bg-surface={!selected}
                class="flex min-h-11 cursor-pointer items-center justify-center rounded-md border px-3 text-sm font-medium hover:bg-subtle sm:min-h-10"
              >
                <input
                  class="sr-only"
                  type="radio"
                  name="annotation-blog-status"
                  value={status}
                  bind:group={draft.blog_status}
                />
                {statusLabels[status]}
              </label>
            {/each}
          </div>
        </fieldset>

        <div class={fieldClass}>
          <div class="flex items-center justify-between gap-2">
            <label for="annotation-link-page">友链页面</label>
            {#if linkPageHref}
              <div class="flex items-center gap-3">
                <button
                  class="cursor-pointer text-xs font-medium text-tint-fg hover:underline"
                  type="button"
                  disabled={!ready}
                  onclick={() => openInPreview(linkPageHref, { focus: true, enableLink: true })}
                  >右窗打开</button
                >
                <a
                  class="text-xs font-medium text-tint-fg hover:underline"
                  href={linkPageHref}
                  target="_blank"
                  rel="noopener noreferrer">新标签页打开 ↗</a
                >
              </div>
            {/if}
          </div>
          <input
            id="annotation-link-page"
            class={inputClass}
            type="url"
            value={linkPageUrl}
            maxlength="2048"
            placeholder="https://example.com/links"
            oninput={(event) => updateLinkPage(event.currentTarget.value)}
          />
        </div>
      </section>

      <AnnotationLinks bind:draft incoming={detail.incoming} {allocateKey} section="outgoing" />

      <section class={sectionClass} aria-labelledby="annotation-save">
        <h2 id="annotation-save" class="text-lg font-semibold">保存标注</h2>
        <label class={fieldClass}
          >变更原因（选填）<input class={inputClass} bind:value={reason} maxlength="2000" /></label
        >
        <div class="flex flex-wrap items-center justify-between gap-3">
          <p class="text-sm text-fg-muted">
            修订号 {detail.revision}{patch ? ' · 有未保存的改动' : ''}{prefetchStatus === 'ready'
              ? ' · 已预加载下一个'
              : ''}
          </p>
          <div class="flex flex-wrap items-center gap-3">
            <button
              class={buttonClass}
              type="submit"
              value="save"
              disabled={!ready || !patch || loading || navigatingNext || conflict}
            >
              {saving ? '保存中…' : '保存标注'}
            </button>
            <button
              class={primaryClass}
              type="submit"
              value="save-and-next"
              disabled={!ready || !patch || loading || navigatingNext || conflict}
            >
              {saving ? '保存中…' : navigatingNext ? '正在抽取…' : '保存并下一个 →'}
            </button>
          </div>
        </div>
      </section>

      <details class="grid min-w-0 gap-4 rounded-md border border-line bg-surface p-4 sm:p-6">
        <summary class="min-h-11 cursor-pointer py-2 text-base font-semibold sm:min-h-10">
          更多信息
          <span class="ml-2 text-sm font-normal text-fg-muted">
            基本信息、分类与标签、站点组件、订阅源、站点地图、指向本站的友链
          </span>
        </summary>
        <div class="grid min-w-0 gap-6 pt-2">
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
            <div class={fieldClass}>
              <span>检测访问范围</span>
              <p>
                {detail.access_scope === null ? '暂无检测结论' : scopeLabels[detail.access_scope]}
              </p>
            </div>
          </section>

          <AnnotationTaxonomy bind:draft {options} {allocateKey} />
          <AnnotationLocations bind:draft {allocateKey} includeLinkPage={false} />
          <AnnotationLinks bind:draft incoming={detail.incoming} {allocateKey} section="incoming" />
        </div>
      </details>
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
