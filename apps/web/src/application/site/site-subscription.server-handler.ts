import { proxyUpstreamText } from '@/application/shared/upstream-proxy.server';

export async function handleSiteSubscriptionRequest(request: Request): Promise<Response> {
  const url = new URL(request.url);

  return proxyUpstreamText(
    `/api/public/subscriptions${url.search}`,
    { method: 'GET' },
    {
      request,
      fallbackMessage: 'Unable to reach subscription service right now.',
    },
  );
}
