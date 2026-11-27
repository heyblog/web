import type { Adapter } from './protocol.ts';

type EdgeFetchInit = RequestInit & {
  readonly eo: {
    readonly resolveOverride: string;
    readonly timeoutSetting: {
      readonly connectTimeout: number;
      readonly readTimeout: number;
      readonly writeTimeout: number;
    };
  };
};

// RequestInit.eo is the documented EdgeOne extension point:
// https://edgeone.ai/document/52690#RequestInitEoProperties
export function edgeOneFetch(
  ingressURL: string,
  nativeFetch: (url: string, init: EdgeFetchInit) => Promise<Response>,
): Adapter['fetch'] {
  const ingressHost = new URL(ingressURL).hostname.replace(/\.$/u, '');
  return async (target, init) => {
    // Requests to the ingress host can enter EdgeOne's CDN path, which ignores
    // resolveOverride. URL normalizes case and IDNs; strip a DNS root dot too.
    if (new URL(target.url).hostname.replace(/\.$/u, '') === ingressHost) return null;
    return nativeFetch(target.url, {
      ...init,
      eo: {
        resolveOverride: target.resolvedIP,
        timeoutSetting: {
          connectTimeout: target.timeoutMs,
          readTimeout: target.timeoutMs,
          writeTimeout: target.timeoutMs,
        },
      },
    });
  };
}
