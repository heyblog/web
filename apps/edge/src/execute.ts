import { type Adapter, type EdgeRequest, type EdgeResult, failure } from './protocol.ts';
import { safeTarget, targetHeaders } from './target.ts';

const binaryChunkSize = 0x8000;

export async function execute(request: EdgeRequest, adapter: Adapter): Promise<EdgeResult> {
  const headers = targetHeaders(request.headers, request.headerEncoding);
  if (request.platform !== adapter.platform || !safeTarget(request) || !headers)
    return failure('unsafe_target', adapter.node);
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), request.timeoutMs);
  try {
    const response = await adapter.fetch(request, {
      method: request.method,
      headers,
      redirect: 'manual',
      signal: controller.signal,
    });
    if (response === null) return failure('unsafe_target', adapter.node);
    const outgoing: Record<string, string> = {};
    for (const key of ['location', 'content-type', 'etag', 'last-modified', 'retry-after']) {
      const value = response.headers.get(key);
      if (value && value.length <= 8192) outgoing[key] = value;
    }
    const reader = response.body?.getReader();
    const chunks: Uint8Array[] = [];
    let length = 0;
    if (reader) {
      try {
        for (;;) {
          const chunk = await reader.read();
          if (chunk.done) break;
          length += chunk.value.byteLength;
          if (length > request.maxBytes) return failure('response_limit', adapter.node);
          chunks.push(chunk.value);
        }
      } finally {
        try {
          await reader.cancel();
        } finally {
          reader.releaseLock();
        }
      }
    }
    const binary: string[] = [];
    for (const chunk of chunks) {
      // Keep spread arguments bounded even if Fetch returns the whole body at once.
      for (let start = 0; start < chunk.length; start += binaryChunkSize) {
        binary.push(String.fromCharCode(...chunk.subarray(start, start + binaryChunkSize)));
      }
    }
    return {
      status: response.status,
      headers: outgoing,
      body: btoa(binary.join('')),
      node: adapter.node,
    };
  } catch (error) {
    if (error instanceof Error)
      return failure(controller.signal.aborted ? 'target_timeout' : 'target_network', adapter.node);
    throw error;
  } finally {
    clearTimeout(timeout);
  }
}
