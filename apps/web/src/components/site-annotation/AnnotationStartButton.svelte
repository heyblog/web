<script lang="ts">
  import { requestNextAnnotation } from '@/api/site-annotation/annotation.browser';
  import {
    annotationDetailPath,
    annotationFailure,
  } from '@/application/site-annotation/annotation-view';

  import { primaryClass } from './annotation.styles';

  let { unreviewed }: { unreviewed: number } = $props();

  let picking = $state(false);
  let notice = $state('');
  let error = $state('');

  async function startAnnotation() {
    if (picking || unreviewed <= 0) return;
    picking = true;
    notice = '';
    error = '';
    try {
      const result = await requestNextAnnotation();
      if (result.kind === 'ready') {
        window.location.assign(annotationDetailPath(result.shortId));
      } else if (result.kind === 'empty') {
        notice = '当前没有待标注的博客。';
      } else {
        error = annotationFailure(result.status, 'load');
      }
    } finally {
      picking = false;
    }
  }
</script>

<div class="grid gap-3">
  <button
    class={`${primaryClass} min-h-12 px-6 text-base sm:w-fit`}
    type="button"
    disabled={picking || unreviewed <= 0}
    onclick={startAnnotation}
  >
    {#if picking}
      正在抽取待标注博客…
    {:else if unreviewed > 0}
      开始标注
    {:else}
      暂无待标注博客
    {/if}
  </button>
  {#if notice}
    <p class="text-sm text-fg-muted" role="status">{notice}</p>
  {/if}
  {#if error}
    <p class="text-sm text-danger-fg" role="alert">{error}</p>
  {/if}
</div>
