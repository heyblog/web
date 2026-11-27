<script lang="ts">
  import { resourceKinds } from '@/api/site-annotation/annotation.types';
  import { feedFormats } from '@/api/site-submission/site-submission.types';
  import type { AnnotationDraft } from '@/application/site-annotation/annotation-draft';
  import { feedFormatLabels, resourceLabels } from '@/application/site-annotation/annotation-view';

  import { buttonClass, fieldClass, inputClass, rowClass, sectionClass } from './annotation.styles';

  let {
    draft = $bindable(),
    allocateKey,
  }: {
    draft: AnnotationDraft;
    allocateKey: () => number;
  } = $props();

  function ensureDefaultFeed() {
    if (draft.feeds.some((feed) => feed.is_default)) return;
    const enabled = draft.feeds.find((feed) => feed.is_enabled);
    if (enabled) enabled.is_default = true;
  }

  function addFeed() {
    draft.feeds.push({
      key: allocateKey(),
      name: '',
      url: '',
      format: 'UNKNOWN',
      is_default: false,
      is_enabled: true,
    });
    ensureDefaultFeed();
  }

  function setEnabled(feed: AnnotationDraft['feeds'][number], enabled: boolean) {
    feed.is_enabled = enabled;
    if (!enabled) feed.is_default = false;
    ensureDefaultFeed();
  }

  function chooseDefault(key: number) {
    for (const feed of draft.feeds) feed.is_default = feed.key === key;
  }

  function removeFeed(key: number) {
    draft.feeds = draft.feeds.filter((entry) => entry.key !== key);
    ensureDefaultFeed();
  }

  function addResource() {
    const kind = resourceKinds.find(
      (kind) => !draft.resources.some((resource) => resource.kind === kind),
    );
    if (kind) draft.resources.push({ key: allocateKey(), kind, url: '' });
  }
</script>

<section class={sectionClass} aria-labelledby="annotation-feeds">
  <div class="flex flex-wrap items-center justify-between gap-3">
    <h2 id="annotation-feeds" class="text-lg font-semibold">订阅源</h2>
    <button class={buttonClass} type="button" onclick={addFeed} disabled={draft.feeds.length >= 8}
      >添加订阅源</button
    >
  </div>
  {#if draft.feeds.length === 0}<p class="text-sm text-fg-muted">暂无订阅源。</p>{/if}
  {#each draft.feeds as feed (feed.key)}
    <div class={rowClass}>
      <label class={fieldClass}
        >名称<input class={inputClass} bind:value={feed.name} maxlength="100" required /></label
      >
      <label class={fieldClass}
        >网址<input
          class={inputClass}
          type="url"
          bind:value={feed.url}
          maxlength="2048"
          required
        /></label
      >
      <label class={fieldClass}
        >格式
        <select class={inputClass} bind:value={feed.format}>
          {#each feedFormats as format (format)}<option value={format}
              >{feedFormatLabels[format]}</option
            >{/each}
        </select>
      </label>
      <div class="flex flex-wrap items-center gap-x-4">
        <label class="flex min-h-11 items-center gap-2 text-sm sm:min-h-10">
          <input
            type="checkbox"
            checked={feed.is_enabled}
            onchange={(event) => setEnabled(feed, event.currentTarget.checked)}
          /> 启用
        </label>
        <label class="flex min-h-11 items-center gap-2 text-sm sm:min-h-10">
          <input
            type="radio"
            name="annotation-default-feed"
            checked={feed.is_default}
            disabled={!feed.is_enabled}
            onchange={() => chooseDefault(feed.key)}
          /> 默认订阅源
        </label>
      </div>
      <button
        class={`${buttonClass} justify-self-start`}
        type="button"
        onclick={() => removeFeed(feed.key)}>移除订阅源</button
      >
    </div>
  {/each}
</section>

<section class={sectionClass} aria-labelledby="annotation-resources">
  <div class="flex flex-wrap items-center justify-between gap-3">
    <h2 id="annotation-resources" class="text-lg font-semibold">资源</h2>
    <button
      class={buttonClass}
      type="button"
      onclick={addResource}
      disabled={draft.resources.length >= resourceKinds.length}>添加资源</button
    >
  </div>
  {#if draft.resources.length === 0}<p class="text-sm text-fg-muted">暂无资源。</p>{/if}
  {#each draft.resources as resource (resource.key)}
    <div class={rowClass}>
      <label class={fieldClass}
        >类型
        <select class={inputClass} bind:value={resource.kind}>
          {#each resourceKinds as kind (kind)}
            <option
              value={kind}
              disabled={draft.resources.some(
                (entry) => entry.key !== resource.key && entry.kind === kind,
              )}>{resourceLabels[kind]}</option
            >
          {/each}
        </select>
      </label>
      <label class={fieldClass}
        >网址<input
          class={inputClass}
          type="url"
          bind:value={resource.url}
          maxlength="2048"
          required
        /></label
      >
      <button
        class={`${buttonClass} justify-self-start`}
        type="button"
        onclick={() => {
          draft.resources = draft.resources.filter((entry) => entry.key !== resource.key);
        }}>移除资源</button
      >
    </div>
  {/each}
</section>
