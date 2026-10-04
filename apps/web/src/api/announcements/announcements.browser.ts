import {
  type AnnouncementFields,
  type AnnouncementList,
  isManagedAnnouncement,
  isRecord,
  type ManagedAnnouncement,
  parseAnnouncementList,
  parseManagedAnnouncement,
  parseRevisions,
} from './announcements.types.ts';

export class AnnouncementTransportError extends Error {
  readonly status: number;
  readonly code: string;
  constructor(status: number, code: string) {
    super(code);
    this.status = status;
    this.code = code;
  }
}

export function createAnnouncementTransport(fetcher: typeof fetch = fetch) {
  async function request(path: string, method = 'GET', body?: object): Promise<Response> {
    let response: Response;
    try {
      response = await fetcher(`/management/announcements/data${path}`, {
        method,
        credentials: 'same-origin',
        cache: 'no-store',
        headers: body
          ? { Accept: 'application/json', 'Content-Type': 'application/json' }
          : { Accept: 'application/json' },
        body: body ? JSON.stringify(body) : undefined,
        signal: AbortSignal.timeout(15_000),
      });
    } catch (error) {
      if (error instanceof Error) throw new AnnouncementTransportError(0, 'network_error');
      throw error;
    }
    if (!response.ok) {
      let code = 'request_failed';
      try {
        const value: unknown = await response.json();
        if (isRecord(value) && typeof value.code === 'string') code = value.code;
      } catch (error) {
        if (!(error instanceof Error)) throw error;
      }
      throw new AnnouncementTransportError(response.status, code);
    }
    return response;
  }

  async function decode<T>(response: Response, parse: (value: unknown) => T | null): Promise<T> {
    let value: unknown;
    try {
      value = await response.json();
    } catch (error) {
      if (error instanceof Error) throw new AnnouncementTransportError(0, 'invalid_response');
      throw error;
    }
    const result = parse(value);
    if (result === null) throw new AnnouncementTransportError(0, 'invalid_response');
    return result;
  }

  return {
    list: async (query = ''): Promise<AnnouncementList<ManagedAnnouncement>> =>
      decode(await request(query), (value) => parseAnnouncementList(value, isManagedAnnouncement)),
    detail: async (id: string) =>
      decode(await request(`/${encodeURIComponent(id)}`), parseManagedAnnouncement),
    create: async (fields: AnnouncementFields) =>
      decode(await request('', 'POST', fields), parseManagedAnnouncement),
    update: async (id: string, fields: AnnouncementFields, rowVersion: string) =>
      decode(
        await request(`/${encodeURIComponent(id)}`, 'PUT', { ...fields, rowVersion }),
        parseManagedAnnouncement,
      ),
    publish: async (
      id: string,
      rowVersion: string,
      startsAt: string | null,
      endsAt: string | null,
    ) =>
      decode(
        await request(`/${encodeURIComponent(id)}/publish`, 'POST', {
          rowVersion,
          startsAt,
          endsAt,
        }),
        parseManagedAnnouncement,
      ),
    archive: async (id: string, rowVersion: string) =>
      decode(
        await request(`/${encodeURIComponent(id)}/archive`, 'POST', { rowVersion }),
        parseManagedAnnouncement,
      ),
    delete: async (id: string, rowVersion: string) => {
      await request(`/${encodeURIComponent(id)}`, 'DELETE', { rowVersion });
    },
    revisions: async (id: string) =>
      decode(await request(`/${encodeURIComponent(id)}/revisions`), parseRevisions),
  };
}
