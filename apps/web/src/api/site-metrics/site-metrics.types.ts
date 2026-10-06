export interface SiteMetricCounts {
  readonly clickCount: number;
  readonly impressionCount: number;
  readonly responseCount: number;
  readonly queryCount: number;
}

export interface SiteImpressionEvent {
  readonly eventId: string;
  readonly shortId: string;
}

export function isSiteMetricCounts(value: unknown): value is SiteMetricCounts {
  if (!value || typeof value !== 'object') return false;
  return ['clickCount', 'impressionCount', 'responseCount', 'queryCount'].every((key) => {
    const count: unknown = Reflect.get(value, key);
    return typeof count === 'number' && Number.isSafeInteger(count) && count >= 0;
  });
}

const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/iu;
const shortIdPattern = /^[0-9A-Za-z]{9}$/u;

export function parseImpressions(value: unknown): readonly SiteImpressionEvent[] | null {
  if (
    !value ||
    typeof value !== 'object' ||
    !('events' in value) ||
    Object.keys(value).length !== 1
  )
    return null;
  if (!Array.isArray(value.events) || value.events.length === 0 || value.events.length > 100)
    return null;
  const events: SiteImpressionEvent[] = [];
  const candidates: readonly unknown[] = value.events;
  for (const item of candidates) {
    if (
      !item ||
      typeof item !== 'object' ||
      Object.keys(item).length !== 2 ||
      !('eventId' in item) ||
      typeof item.eventId !== 'string' ||
      !uuidPattern.test(item.eventId) ||
      !('shortId' in item) ||
      typeof item.shortId !== 'string' ||
      !shortIdPattern.test(item.shortId)
    )
      return null;
    events.push({ eventId: item.eventId, shortId: item.shortId });
  }
  return events;
}
