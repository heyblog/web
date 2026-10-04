<script lang="ts">
  import type { AnnouncementRevision } from '../../../api/announcements/announcements.types.ts';
  import AnnouncementInline from '../../announcements/AnnouncementInline.svelte';

  let { revisions }: { revisions: readonly AnnouncementRevision[] | null } = $props();
</script>

<section class="mt-10 border-t border-line pt-6" aria-label="修订历史">
  <h2 class="text-lg font-semibold">修订历史</h2>
  {#if revisions === null}<p class="mt-4 text-sm text-danger-fg">修订历史暂时无法加载。</p>
  {:else if revisions.length === 0}<p class="mt-4 text-sm text-fg-muted">暂无修订记录。</p>
  {:else}
    {#each revisions as revision (revision.revision)}
      <details class="border-b border-line py-4">
        <summary class="cursor-pointer text-sm font-medium"
          >版本 {revision.revision}<time
            class="ml-3 font-normal text-fg-muted"
            datetime={revision.changedAt}
            >{new Date(revision.changedAt).toLocaleString('zh-CN', {
              timeZone: 'Asia/Shanghai',
            })}</time
          ></summary
        >
        <div class="mt-4 grid gap-3 text-sm">
          <h3 class="font-semibold wrap-anywhere">{revision.title}</h3>
          {#if revision.bodyMarkdown}<p class="leading-6">
              <AnnouncementInline source={revision.bodyMarkdown} />
            </p>{/if}
          <dl class="grid gap-2 text-fg-muted">
            <div>
              <dt class="inline">类型：</dt>
              <dd class="inline">{revision.kind === 'MAIN' ? '主公告' : '横幅公告'}</dd>
            </div>
            <div>
              <dt class="inline">优先级：</dt>
              <dd class="inline">{revision.priority}</dd>
            </div>
            <div>
              <dt class="inline">开始时间：</dt>
              <dd class="inline">
                {revision.startsAt
                  ? new Date(revision.startsAt).toLocaleString('zh-CN', {
                      timeZone: 'Asia/Shanghai',
                    })
                  : '未设置'}
              </dd>
            </div>
            <div>
              <dt class="inline">结束时间：</dt>
              <dd class="inline">
                {revision.endsAt
                  ? new Date(revision.endsAt).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai' })
                  : '无'}
              </dd>
            </div>
            {#if revision.actionType !== 'NONE'}<div>
                <dt class="inline">链接：</dt>
                <dd class="inline wrap-anywhere">
                  {revision.actionLabel} · {revision.actionPath ?? revision.actionExternalUrl}
                </dd>
              </div>{/if}
          </dl>
        </div>
      </details>
    {/each}
  {/if}
</section>
