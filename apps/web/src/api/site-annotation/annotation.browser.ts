import type { AnnotationDetail, AnnotationPatch } from './annotation.types.ts';
import { annotationAPIPath, isAnnotationID } from './annotation-contract.ts';
import { parseAnnotationDetail } from './annotation-responses.ts';

export type AnnotationResult =
  | { readonly kind: 'ready'; readonly detail: AnnotationDetail }
  | { readonly kind: 'error'; readonly status: number; readonly code?: string };

export async function requestAnnotation(
  shortId: string,
  input?: AnnotationPatch,
): Promise<AnnotationResult> {
  if (!isAnnotationID(shortId) || shortId === 'options') return { kind: 'error', status: 404 };
  try {
    const response = await fetch(`${annotationAPIPath}/${shortId}`, {
      method: input ? 'PATCH' : 'GET',
      headers: input
        ? { Accept: 'application/json', 'Content-Type': 'application/json' }
        : { Accept: 'application/json' },
      ...(input ? { body: JSON.stringify(input) } : {}),
      credentials: 'same-origin',
      mode: 'same-origin',
      redirect: 'error',
      cache: 'no-store',
      signal: AbortSignal.timeout(15_000),
    });
    if (!response.ok) {
      let code: unknown;
      if (response.status === 409) {
        const payload: unknown = await response.json().catch(() => null);
        if (typeof payload === 'object' && payload !== null && 'code' in payload)
          code = payload.code;
      }
      return {
        kind: 'error',
        status: response.status,
        ...(typeof code === 'string' ? { code } : {}),
      };
    }
    const detail = parseAnnotationDetail(await response.json());
    return detail.short_id === shortId ? { kind: 'ready', detail } : { kind: 'error', status: 502 };
  } catch (error) {
    if (!(error instanceof Error)) throw error;
    return { kind: 'error', status: 502 };
  }
}
