import { loadWebServerConfig, type WebServerConfig } from '../../config.server.ts';
import { forwardClientAddress } from '../api/client-ip.server.ts';
import { apiWebTokenHeader } from '../api/endpoint.server.ts';
import type { SiteProfile } from '../site-profile/site-profile.server.ts';

export type SiteIconResult =
  | { readonly kind: 'ready'; readonly dataUrl: string }
  | { readonly kind: 'missing' }
  | { readonly kind: 'unavailable' };

interface IconDependencies {
  readonly fetch?: typeof fetch;
  readonly loadConfig?: () => WebServerConfig;
}

const maximumBytes = 128 * 1024;
const pngSignature = [137, 80, 78, 71, 13, 10, 26, 10];

async function readIcon(response: Response): Promise<Uint8Array | null> {
  const reader = response.body?.getReader();
  if (!reader) return null;
  const chunks: Uint8Array[] = [];
  let length = 0;
  try {
    while (true) {
      const chunk = await reader.read();
      if (chunk.done) break;
      length += chunk.value.byteLength;
      if (length > maximumBytes) {
        await reader.cancel();
        return null;
      }
      chunks.push(chunk.value);
    }
  } finally {
    reader.releaseLock();
  }
  const bytes = Buffer.concat(chunks);
  if (bytes.length < 24 || !pngSignature.every((value, index) => bytes[index] === value))
    return null;
  const width = bytes.readUInt32BE(16);
  const height = bytes.readUInt32BE(20);
  return width > 0 && width <= 128 && height > 0 && height <= 128 ? bytes : null;
}

export async function loadSiteIcon(
  profile: SiteProfile,
  request: Request,
  dependencies: IconDependencies = {},
): Promise<SiteIconResult> {
  if (!profile.iconHash) return { kind: 'missing' };
  try {
    const config = (dependencies.loadConfig ?? loadWebServerConfig)();
    const headers = new Headers({ Accept: 'image/png', [apiWebTokenHeader]: config.apiWebToken });
    forwardClientAddress(request, headers);
    const response = await (dependencies.fetch ?? fetch)(
      new URL(`/sites/id/${encodeURIComponent(profile.shortId)}/icon`, config.apiBaseUrl),
      {
        headers,
        redirect: 'error',
        signal: AbortSignal.any([request.signal, AbortSignal.timeout(2_000)]),
      },
    );
    if (response.status === 404) {
      await response.body?.cancel();
      return { kind: 'missing' };
    }
    if (
      !response.ok ||
      response.headers.get('content-type')?.split(';')[0] !== 'image/png' ||
      response.headers.get('etag') !== `"${profile.iconHash}"`
    ) {
      await response.body?.cancel();
      return { kind: 'unavailable' };
    }
    const bytes = await readIcon(response);
    return bytes
      ? { kind: 'ready', dataUrl: `data:image/png;base64,${Buffer.from(bytes).toString('base64')}` }
      : { kind: 'unavailable' };
  } catch (error) {
    if (error instanceof Error) return { kind: 'unavailable' };
    throw error;
  }
}
