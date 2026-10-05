import { parseSlugJob, parseSlugJobs, type SlugSelection } from './slug-jobs.types.ts';
import { requestManagement } from './taxonomy.browser.ts';
const base = '/management/tags/data/slug-jobs';
export const listSlugJobs = (signal?: AbortSignal) =>
  requestManagement(base, 'GET', parseSlugJobs, undefined, signal);
export const readSlugJob = (id: string, signal?: AbortSignal) =>
  requestManagement(`${base}/${encodeURIComponent(id)}`, 'GET', parseSlugJob, undefined, signal);
export const createSlugJob = (selection: SlugSelection) =>
  requestManagement(base, 'POST', parseSlugJob, { selection });
export const controlSlugJob = (
  id: string,
  action: 'pause' | 'resume' | 'cancel' | 'retry',
  revision: string,
) =>
  requestManagement(`${base}/${encodeURIComponent(id)}/control`, 'POST', parseSlugJob, {
    action,
    expected_revision: revision,
  });
export const editSlugJob = (
  id: string,
  revision: string,
  items: readonly { tag_id: string; slug: string }[],
) =>
  requestManagement(`${base}/${encodeURIComponent(id)}/items`, 'PATCH', parseSlugJob, {
    items,
    expected_revision: revision,
  });
export const applySlugJob = (id: string, revision: string, tagIDs: readonly string[]) =>
  requestManagement(`${base}/${encodeURIComponent(id)}/apply`, 'POST', parseSlugJob, {
    tag_ids: tagIDs,
    expected_revision: revision,
  });
