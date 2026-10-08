<script lang="ts">
  interface Props {
    auditId: string;
  }
  let { auditId }: Props = $props();
</script>

<div class="grid gap-4" role="status">
  <h2 class="text-xl font-semibold">申请已提交</h2>
  <a class="text-tint-fg underline" href={`/dashboard/submissions/${auditId}`}>查看审核进度</a>
</div>
