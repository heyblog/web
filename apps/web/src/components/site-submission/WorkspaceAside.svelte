<script lang="ts">
  import type { SubmissionPage } from '@/application/site-submission/site-submission.service';

  type AsidePage = Exclude<SubmissionPage, 'query'>;

  let {
    activePage = 'create',
  }: {
    activePage?: AsidePage;
  } = $props();

  const asideContent: Record<
    AsidePage,
    {
      eyebrow: string;
      title: string;
      intro: string;
      items: string[];
      footnote: string;
    }
  > = {
    create: {
      eyebrow: '新增须知',
      title: '请提交完整、可公开验证的站点资料',
      intro:
        '我们将依据收录标准审核提交站点，确保符合公开展示要求。如对标准有疑问或建议，欢迎随时联系我们，我们会持续优化流程与规范。',
      items: [
        '遵守中华人民共和国相关法律法规，内容健康向上，符合公共道德；',
        '符合博客的基本属性，例如内容具有个人见解和知识沉淀，保持独立运营维护；',
        '请提供完整准确信息以便快速审核，尤其是域名、订阅地址和展示名称等核心字段；',
        '可选字段越完整，越有利于站点在目录中的展示质量和后续维护；',
        '更多信息请参考文档中收录标准和常见问题，如有疑问欢迎联系我们。',
      ],
      footnote: '标有 ✱ 的为必填项，建议尽量完善可选项以更好展示博客。',
    },
    update: {
      eyebrow: '修订提示',
      title: '请确保提交内容完整且反映站点当前状态',
      intro:
        '我们欢迎任意人员对任意站点提交修订建议，如果需要反馈，请优先使用博客详情页面、博客卡片、文章卡片中的反馈入口，以便我们及时收到并处理相关信息。',
      items: [
        '修改时，请一次性提交完整的修改内容；',
        '请确保提交内容准确反映站点当前状态，例如站点改版迁移后请更新官网和订阅地址，内容方向调整后请更新简介和标签等；',
        '请注明修订原因，例如改版迁移、内容方向调整、订阅地址变更等。',
      ],
      footnote: '请避免重复提交相同修订。',
    },
    delete: {
      eyebrow: '删除说明',
      title: '删除用于下架站点，不用于修正普通资料错误。',
      intro:
        '如果站点只是信息过时、订阅异常或展示项需要更正，请优先使用修订表单；删除申请更适合处理失效、重复或不再公开展示的站点。',
      items: [
        '请说明删除原因，例如站点关闭、域名失效、重复收录、作者主动申请下架等。',
        '审核通过后，站点将从目录中下架，但相关信息会留存，避免二次收录；',
      ],
      footnote: '删除申请进入人工审核后不会立即生效，请避免把它当作即时隐藏操作。',
    },
    restore: {
      eyebrow: '恢复说明',
      title: '恢复用于重新公开已下线站点。',
      intro:
        '恢复申请适用于曾经收录、当前已下线但已经恢复正常访问和公开展示条件的站点。请结合当前状态填写恢复说明，便于审核判断。',
      items: [
        '请确认站点当前已经恢复可访问，且内容仍符合公开展示要求。',
        '请说明本次恢复原因，例如站点已恢复更新、域名问题已修复、误下线后申请恢复等。',
        '恢复只重新开启公开展示，不适合处理普通资料修正；如需改资料，请改用修订表单。',
        '如果需要修改信息，请在恢复申请通过后提交修订申请。',
      ],
      footnote: '恢复申请会进入人工审核，请避免在短时间内重复提交相同恢复请求。',
    },
  };

  const content = $derived(asideContent[activePage]);
</script>

<aside class="order-1 space-y-5 rounded-md border border-(--color-line-med) p-6 sm:p-7 lg:order-2">
  <div>
    <p class="font-mono text-[11px] tracking-[0.18em] text-(--color-fg-3) uppercase">
      {content.eyebrow}
    </p>
    <p class="mt-3 text-base leading-7 text-(--color-fg)">{content.title}</p>
  </div>

  <div class="space-y-4 text-sm leading-7 text-(--color-fg-2)">
    <p>{content.intro}</p>

    <ul class="space-y-2 border-t border-(--color-line) pt-4">
      {#each content.items as item (item)}
        <li class="flex gap-3">
          <span
            class="mt-[0.7rem] h-1.5 w-1.5 shrink-0 rounded-full bg-red-700 dark:bg-red-400"
            aria-hidden="true"
          ></span>
          <span>{item}</span>
        </li>
      {/each}
    </ul>

    <p class="border-t border-(--color-line) pt-4 text-red-700 dark:text-red-400">
      {content.footnote}
    </p>
  </div>
</aside>
