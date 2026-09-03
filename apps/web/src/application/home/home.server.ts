import { type ApiJsonResult, fetchApiJson } from '@/application/api/client.server';
import type { HomeView } from '@/application/home/home.shared';

export function loadHome(request?: Request): Promise<ApiJsonResult<HomeView>> {
  return fetchApiJson<HomeView>('/home', { request, signal: request?.signal });
}
