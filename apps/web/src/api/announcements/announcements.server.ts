import { requestAuthAPI } from '../auth/auth.server.ts';
import { type ApiJsonResult, fetchApiJson } from '../transport/client.server.ts';

import {
  type AnnouncementList,
  isManagedAnnouncement,
  isPublicAnnouncement,
  isRecord,
  type ManagedAnnouncement,
  parseAnnouncementList,
  parseManagedAnnouncement,
  parseRevisions,
  type PublicAnnouncement,
} from './announcements.types.ts';

async function readAnnouncementJSON(response: Response): Promise<unknown> {
  try {
    return await response.json();
  } catch (error) {
    if (error instanceof SyntaxError || error instanceof TypeError) return null;
    throw error;
  }
}

export async function loadPublicAnnouncements(
  request: Request,
  page: number,
): Promise<ApiJsonResult<AnnouncementList<PublicAnnouncement>>> {
  const result = await fetchApiJson<unknown>(`/announcements?page=${page}&pageSize=20`, {
    request,
    signal: request.signal,
  });
  if (result.kind !== 'success') return result;
  const list = parseAnnouncementList(result.data, isPublicAnnouncement);
  return list ? { kind: 'success', data: list } : { kind: 'unavailable' };
}

export async function loadPublicAnnouncement(
  request: Request,
  id: string,
): Promise<ApiJsonResult<PublicAnnouncement>> {
  const result = await fetchApiJson<unknown>(`/announcements/${encodeURIComponent(id)}`, {
    request,
    signal: request.signal,
  });
  if (result.kind !== 'success') return result;
  return isRecord(result.data) && isPublicAnnouncement(result.data.announcement)
    ? { kind: 'success', data: result.data.announcement }
    : { kind: 'unavailable' };
}

export async function loadAnnouncementBanner(request: Request): Promise<PublicAnnouncement | null> {
  const result = await fetchApiJson<unknown>('/announcements/banner', {
    request,
    signal: request.signal,
  });
  return result.kind === 'success' &&
    isRecord(result.data) &&
    isPublicAnnouncement(result.data.announcement)
    ? result.data.announcement
    : null;
}

export async function loadManagedAnnouncements(request: Request, query: string) {
  const response = await requestAuthAPI(request, `/management/announcements${query}`);
  if (!response.ok) return null;
  const value = await readAnnouncementJSON(response);
  return parseAnnouncementList(value, isManagedAnnouncement);
}

export async function loadManagedAnnouncement(
  request: Request,
  id: string,
): Promise<{ status: number; announcement: ManagedAnnouncement | null }> {
  const response = await requestAuthAPI(
    request,
    `/management/announcements/${encodeURIComponent(id)}`,
  );
  if (!response.ok) return { status: response.status, announcement: null };
  const value = await readAnnouncementJSON(response);
  const announcement = parseManagedAnnouncement(value);
  return { status: announcement ? 200 : 502, announcement };
}

export async function loadAnnouncementRevisions(request: Request, id: string) {
  const response = await requestAuthAPI(
    request,
    `/management/announcements/${encodeURIComponent(id)}/revisions`,
  );
  if (!response.ok) return null;
  const value = await readAnnouncementJSON(response);
  return parseRevisions(value);
}
