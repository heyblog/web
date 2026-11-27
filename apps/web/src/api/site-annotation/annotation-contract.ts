import { annotationFilters } from './annotation.types.ts';

export const annotationAPIPath = '/api/web/management/site-annotations';
export const annotationSearchLimit = 160;
export const annotationBodyLimit = 256 * 1024;

export function isAnnotationID(value: string | undefined): value is string {
  return Boolean(value && /^[a-zA-Z0-9_-]{1,64}$/u.test(value));
}

export function annotationStatus(value: string | null): (typeof annotationFilters)[number] {
  return annotationFilters.find((status) => status === value) ?? 'UNREVIEWED';
}

export function annotationQuery(
  source: URLSearchParams,
  scope: 'list' | 'history',
): URLSearchParams {
  const query = new URLSearchParams();
  if (scope === 'list') {
    query.set('status', annotationStatus(source.get('status')));
    const q = source.get('q')?.trim().slice(0, annotationSearchLimit);
    if (q) query.set('q', q);
  }
  const page = source.get('page');
  if (page && /^(?:[1-9]\d{0,5}|1000000)$/u.test(page)) query.set('page', page);
  return query;
}

export function annotationTarget(
  path: string,
  method: string,
  query: URLSearchParams,
): string | null {
  const [id, child, ...rest] = path.split('/');
  if (rest.length) return null;
  if (method === 'GET') {
    if (path === '') return `?${annotationQuery(query, 'list')}`;
    if (path === 'options') return '/options';
    if (isAnnotationID(id) && id !== 'options') {
      if (child === undefined) return `/${id}`;
      if (child === 'history') {
        const parameters = annotationQuery(query, 'history');
        return `/${id}/history${parameters.size ? `?${parameters}` : ''}`;
      }
    }
  }
  if (method === 'PATCH' && isAnnotationID(id) && id !== 'options' && child === undefined)
    return `/${id}`;
  return null;
}
