<script lang="ts">
  interface Props {
    currentStep: number;
    stepCount: number;
    pending: boolean;
    checking: boolean;
    onback: () => void;
    onnext: () => void | Promise<void>;
  }
  let { currentStep, stepCount, pending, checking, onback, onnext }: Props = $props();
</script>

<div class="flex flex-wrap justify-between gap-3 border-t border-line pt-5">
  <button
    class="min-h-11 rounded-sm border border-line-strong px-4 font-medium disabled:opacity-50"
    type="button"
    disabled={currentStep === 0 || pending}
    onclick={onback}>上一步</button
  >
  {#if currentStep < stepCount - 1}
    <button
      class="min-h-11 rounded-sm bg-primary px-5 font-semibold text-primary-fg disabled:pointer-events-none disabled:opacity-50"
      type="button"
      disabled={checking || pending}
      onclick={onnext}>下一步</button
    >
  {:else}
    <button
      class="min-h-11 rounded-sm bg-primary px-5 font-semibold text-primary-fg disabled:opacity-50"
      type="submit"
      disabled={pending}>{pending ? '提交中…' : '提交申请'}</button
    >
  {/if}
</div>
