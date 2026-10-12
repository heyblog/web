import { request as requestHttp } from 'node:http';
import { request as requestHttps } from 'node:https';
import { Readable } from 'node:stream';

// Native HTTP keeps the backup's explicit deadline authoritative while streaming.
export const requestBackup: typeof fetch = async (input, init) => {
  const source = new Request(input, init);
  const url = new URL(source.url);
  return new Promise<Response>((resolve, reject) => {
    const reader = source.body?.getReader();
    const request = (url.protocol === 'https:' ? requestHttps : requestHttp)(url, {
      method: source.method,
      headers: Object.fromEntries(source.headers),
      signal: source.signal,
    });
    request.on('error', reject);
    request.once('close', () => {
      void reader?.cancel().catch(reject);
    });
    request.once('response', (upstream) => {
      void reader?.cancel().catch(reject);
      const headers = new Headers();
      for (let index = 0; index < upstream.rawHeaders.length; index += 2) {
        const name = upstream.rawHeaders[index];
        const value = upstream.rawHeaders[index + 1];
        if (name !== undefined && value !== undefined) headers.append(name, value);
      }
      const status = upstream.statusCode ?? 502;
      if (status === 204 || status === 205 || status === 304) {
        upstream.resume();
        resolve(new Response(null, { status, headers }));
        return;
      }
      const responseReader = Readable.toWeb(upstream).getReader();
      const body = new ReadableStream<Uint8Array>({
        async pull(controller) {
          const chunk = await responseReader.read();
          if (chunk.done) {
            controller.close();
            return;
          }
          const bytes: unknown = chunk.value;
          if (!(bytes instanceof Uint8Array)) throw new TypeError('Expected backup response bytes');
          controller.enqueue(bytes);
        },
        async cancel(reason: unknown) {
          request.destroy();
          await responseReader.cancel(reason);
        },
      });
      resolve(new Response(body, { status, headers }));
    });
    async function upload() {
      if (reader) {
        while (!request.destroyed) {
          const chunk = await reader.read();
          if (chunk.done) break;
          await new Promise<void>((resolveWrite, rejectWrite) => {
            request.write(chunk.value, (error) => (error ? rejectWrite(error) : resolveWrite()));
          });
        }
      }
      request.end();
    }
    void upload().catch((error: unknown) => {
      request.destroy(error instanceof Error ? error : new Error('Backup upload failed'));
      reject(error);
    });
  });
};
