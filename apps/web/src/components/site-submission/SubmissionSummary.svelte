<script lang="ts">
  import type { EditableSubmission } from '@/application/site-submission/site-submission.browser';

  interface Props {
    form: EditableSubmission;
  }

  let { form }: Props = $props();
</script>

<aside class="sticky top-24 hidden rounded-md border border-line bg-subtle p-4 text-sm lg:block">
  <h2 class="font-semibold">申请摘要</h2>
  <dl class="mt-3 grid gap-3">
    <div>
      <dt class="text-fg">站点</dt>
      <dd class="mt-1 wrap-break-word">{form.name || '尚未填写'}</dd>
    </div>
    <div>
      <dt class="text-fg">标签</dt>
      <dd class="mt-1">{form.tags.length || '尚未选择'}</dd>
    </div>
    <div>
      <dt class="text-fg">程序</dt>
      <dd class="mt-1">{form.program.kind === 'none' ? '未填写' : form.program.name}</dd>
    </div>
  </dl>
</aside>
