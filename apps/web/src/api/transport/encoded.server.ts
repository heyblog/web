import { request as requestHttp } from 'node:http';
import { request as requestHttps } from 'node:https';
import { Readable } from 'node:stream';

// Fetch decodes Content-Encoding. This transport keeps large encoded responses
// intact through the same-origin proxy, including cancellation while streaming.
export function requestEncoded(url: URL, headers: Headers, signal: AbortSignal): Promise<Response> {
  return new Promise((resolve, reject) => {
    const request = (url.protocol === 'https:' ? requestHttps : requestHttp)(
      url,
      {
        method: 'GET',
        headers: Object.fromEntries(headers),
        signal,
      },
      (upstream) => {
        const responseHeaders = new Headers();
        for (let i = 0; i < upstream.rawHeaders.length; i += 2) {
          const key = upstream.rawHeaders[i];
          const value = upstream.rawHeaders[i + 1];
          if (key !== undefined && value !== undefined) responseHeaders.append(key, value);
        }
        const status = upstream.statusCode ?? 502;
        const body =
          status === 204 || status === 205 || status === 304 ? null : responseBody(upstream);
        if (body === null) upstream.resume();
        resolve(
          new Response(body, {
            status,
            headers: responseHeaders,
          }),
        );
      },
    );
    request.on('error', reject);
    request.end();
  });
}

function responseBody(upstream: Readable): ReadableStream<Uint8Array> {
  const reader = Readable.toWeb(upstream).getReader();
  return new ReadableStream<Uint8Array>({
    async pull(controller) {
      const result = await reader.read();
      if (result.done) {
        controller.close();
        return;
      }
      const value: unknown = result.value;
      if (!(value instanceof Uint8Array)) throw new TypeError('Expected HTTP response bytes');
      controller.enqueue(value);
    },
    async cancel(reason: unknown) {
      await reader.cancel(reason);
    },
  });
}
