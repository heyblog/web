import { parseModels, parseSettings } from './system-settings.types.ts';
import {
  isRecord,
  parsePreview,
  parseSlug,
  parseTaxonomy,
  type SlugInput,
  type TaxonomyChange,
} from './taxonomy.types.ts';

export type ManagementResult<T> =
  { readonly ok: true; readonly value: T } | { readonly ok: false; readonly code: string };

export async function requestManagement<T>(
  path: string,
  method: string,
  parser: (value: unknown) => T | null,
  body?: unknown,
  signal?: AbortSignal,
): Promise<ManagementResult<T>> {
  try {
    const response = await fetch(path, {
      method,
      credentials: 'same-origin',
      redirect: 'error',
      headers:
        body === undefined
          ? { Accept: 'application/json' }
          : { Accept: 'application/json', 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
      signal: signal
        ? AbortSignal.any([signal, AbortSignal.timeout(25_000)])
        : AbortSignal.timeout(25_000),
    });
    const payload: unknown = await response.json();
    if (!response.ok)
      return {
        ok: false,
        code:
          isRecord(payload) && typeof payload.code === 'string' ? payload.code : 'request_failed',
      };
    const value = parser(payload);
    return value === null ? { ok: false, code: 'invalid_response' } : { ok: true, value };
  } catch (error) {
    if (!(error instanceof Error)) throw error;
    return { ok: false, code: 'network_error' };
  }
}

export const readTaxonomy = () => requestManagement('/management/tags/data', 'GET', parseTaxonomy);
export const saveTag = (id: string, body: Readonly<Record<string, unknown>>) =>
  requestManagement(
    `/management/tags/data${id ? `/${encodeURIComponent(id)}` : ''}`,
    id ? 'PUT' : 'POST',
    parseTaxonomy,
    body,
  );
export const deleteTag = (id: string, revision: string) =>
  requestManagement(`/management/tags/data/${encodeURIComponent(id)}`, 'DELETE', parseTaxonomy, {
    expected_revision: revision,
  });
export const previewChange = (body: TaxonomyChange) =>
  requestManagement('/management/tags/data/changes/preview', 'POST', parsePreview, body);
export const applyChange = (body: TaxonomyChange, fingerprint: string) =>
  requestManagement('/management/tags/data/changes/apply', 'POST', parseTaxonomy, {
    ...body,
    fingerprint,
  });
export const generateSlug = (body: SlugInput) =>
  requestManagement('/management/tags/slug-generation', 'POST', parseSlug, body);
export const readSettings = () =>
  requestManagement('/management/system-settings/data', 'GET', parseSettings);
export const readModels = () =>
  requestManagement('/management/system-settings/models', 'GET', parseModels);
export const saveSettings = (modelID: string, revision: string) =>
  requestManagement('/management/system-settings/data', 'PUT', parseSettings, {
    model_id: modelID,
    expected_revision: revision,
  });

export const createCascade = (body: Readonly<Record<string, unknown>>) =>
  requestManagement('/management/tags/data/cascades', 'POST', parseTaxonomy, body);

export const saveLabel = (
  tagID: string,
  labelID: string,
  body: Readonly<Record<string, unknown>>,
) =>
  requestManagement(
    `/management/tags/data/${encodeURIComponent(tagID)}/labels${labelID ? `/${encodeURIComponent(labelID)}` : ''}`,
    labelID ? 'PUT' : 'POST',
    parseTaxonomy,
    body,
  );
export const deleteLabel = (tagID: string, labelID: string, revision: string) =>
  requestManagement(
    `/management/tags/data/${encodeURIComponent(tagID)}/labels/${encodeURIComponent(labelID)}`,
    'DELETE',
    parseTaxonomy,
    { expected_revision: revision },
  );
export const setDefaultLabel = (tagID: string, labelID: string, revision: string) =>
  requestManagement(
    `/management/tags/data/${encodeURIComponent(tagID)}/default-label`,
    'POST',
    parseTaxonomy,
    { label_id: labelID, expected_revision: revision },
  );
