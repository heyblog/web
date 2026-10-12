<script lang="ts">
  import type { ClaimChallenge } from '@/api/site-management/claims.types';
  import { buttonClass, inputClass } from '@/components/management/api-keys/api-keys.styles';
  let { challenge }: { challenge: ClaimChallenge } = $props();
  let notice = $state('');
  async function copy(value: string) {
    try {
      await navigator.clipboard.writeText(value);
      notice = '已复制。';
    } catch (error) {
      if (error instanceof Error) notice = '复制失败，请选中内容手动复制。';
      else throw error;
    }
  }
  function download(content: string) {
    const url = URL.createObjectURL(new Blob([content], { type: 'text/plain;charset=utf-8' }));
    const link = document.createElement('a');
    link.href = url;
    link.download = 'heyblog-site-verification.txt';
    link.click();
    URL.revokeObjectURL(url);
  }
</script>

{#if challenge.instructions}
  {@const proof = challenge.instructions}
  <section class="grid gap-4 rounded-md border border-line p-5" aria-labelledby="claim-proof-title">
    <h2 id="claim-proof-title" class="text-lg font-semibold">发布验证配置</h2>
    {#if challenge.claim.method === 'DNS_TXT'}
      <p class="text-sm text-fg-muted">在域名服务商处添加 TXT 记录。记录生效后，返回此处检查。</p>
      <label class="grid gap-2 text-sm"
        >完整记录名<input class="{inputClass} font-mono" readonly value={proof.dns_name} /></label
      ><button class="{buttonClass} w-fit" type="button" onclick={() => void copy(proof.dns_name)}
        >复制记录名</button
      >
      <label class="grid gap-2 text-sm"
        >记录值<textarea
          class="{inputClass} min-h-20 py-3 font-mono"
          readonly
          value={proof.dns_value}></textarea></label
      ><button class="{buttonClass} w-fit" type="button" onclick={() => void copy(proof.dns_value)}
        >复制记录值</button
      >
    {:else if challenge.claim.method === 'META'}
      <p class="text-sm text-fg-muted">
        将标签放入主页的 head 中，发布后检查。标签需要包含在服务端返回的 HTML 中。
      </p>
      <label class="grid gap-2 text-sm"
        >Meta 标签<textarea class="{inputClass} min-h-24 py-3 font-mono" readonly value={proof.meta}
        ></textarea></label
      ><button class="{buttonClass} w-fit" type="button" onclick={() => void copy(proof.meta)}
        >复制标签</button
      >
    {:else if challenge.claim.method === 'FILE'}
      <p class="text-sm text-fg-muted">下载验证文件并上传到以下地址，保持文件名和内容不变。</p>
      <p class="text-sm break-all">{proof.file_url}</p>
      <div class="flex flex-wrap gap-3">
        <button class={buttonClass} type="button" onclick={() => download(proof.file_content)}
          >下载验证文件</button
        ><button class={buttonClass} type="button" onclick={() => void copy(proof.file_url)}
          >复制发布地址</button
        >
      </div>
      <label class="grid gap-2 text-sm"
        >文件内容<textarea
          class="{inputClass} min-h-20 py-3 font-mono"
          readonly
          value={proof.file_content}></textarea></label
      >
    {/if}
    {#if notice}<p class="text-sm text-fg-muted" role="status">{notice}</p>{/if}
  </section>
{/if}
