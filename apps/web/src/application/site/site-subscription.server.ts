import { fetchApiJson } from '../api/api.server';

import type { SiteSubscriptionResult } from './site-subscription.models';

interface SubscriptionPayload {
  ok: boolean;
  data: SiteSubscriptionResult;
}

export const readSiteSubscriptionArchive = async (
  page: number,
  pageSize: number,
): Promise<SiteSubscriptionResult | null> => {
  const payload = await fetchApiJson<SubscriptionPayload>(
    `/api/public/subscriptions?page=${page}&pageSize=${pageSize}`,
  );

  return payload?.data ?? null;
};
