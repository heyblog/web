<script lang="ts">
  import { IconDownload, IconX } from '@tabler/icons-svelte';

  interface Props {
    readonly imageAlt: string;
    readonly imagePath: string;
  }

  let { imageAlt, imagePath }: Props = $props();
  let dialog: HTMLDialogElement;
  let trigger: HTMLButtonElement;
  let previousBodyOverflow = '';

  function openPreview() {
    if (dialog.open) return;
    previousBodyOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    dialog.showModal();
  }

  function closePreview() {
    if (dialog.open) dialog.close();
  }

  function handleBackdropClick(event: MouseEvent) {
    if (event.target !== dialog) return;
    const bounds = dialog.getBoundingClientRect();
    const inside =
      event.clientX >= bounds.left &&
      event.clientX <= bounds.right &&
      event.clientY >= bounds.top &&
      event.clientY <= bounds.bottom;
    if (!inside) closePreview();
  }

  function handleCancel(event: Event) {
    event.preventDefault();
    closePreview();
  }

  function handleClosed() {
    document.body.style.overflow = previousBodyOverflow;
    trigger.focus({ preventScroll: true });
  }
</script>

<button
  bind:this={trigger}
  class="inline-flex min-h-11 w-full items-center justify-center gap-2 rounded-md border border-line-strong bg-surface px-4 text-sm font-medium whitespace-nowrap transition-[background-color,scale] duration-(--motion-fast) hover:bg-subtle active:scale-96 motion-reduce:scale-100 motion-reduce:transform-none sm:min-h-10"
  type="button"
  data-site-card-trigger
  aria-haspopup="dialog"
  aria-controls="site-card-preview"
  onclick={openPreview}
>
  <IconDownload aria-hidden="true" size={16} stroke={1.8} />生成名片
</button>

<dialog
  bind:this={dialog}
  id="site-card-preview"
  data-site-card-dialog
  class="fixed inset-0 m-auto max-h-[calc(100dvh-2rem)] w-[min(calc(100%-2rem),30rem)] overflow-y-auto rounded-md border border-line-strong bg-surface p-0 text-fg shadow-md outline-none backdrop:bg-overlay"
  aria-labelledby="site-card-preview-title"
  aria-describedby="site-card-preview-description"
  onclick={handleBackdropClick}
  oncancel={handleCancel}
  onclose={handleClosed}
>
  <div class="p-4 sm:p-6">
    <header class="flex min-w-0 items-start justify-between gap-4">
      <div class="min-w-0">
        <h2 class="text-xl font-semibold" id="site-card-preview-title">站点名片</h2>
        <p class="mt-1 text-sm/6 text-fg-muted" id="site-card-preview-description">
          长按或右键图片即可保存。
        </p>
      </div>
      <button
        class="inline-flex size-11 shrink-0 items-center justify-center rounded-sm text-fg-muted transition-colors duration-(--motion-fast) hover:bg-subtle hover:text-fg sm:size-10"
        type="button"
        aria-label="关闭名片预览"
        onclick={closePreview}
      >
        <IconX aria-hidden="true" size={18} stroke={1.8} />
      </button>
    </header>
    <img
      class="mt-4 h-auto w-full rounded-md bg-subtle"
      src={imagePath}
      alt={imageAlt}
      width="1200"
      height="630"
      loading="lazy"
    />
  </div>
</dialog>
