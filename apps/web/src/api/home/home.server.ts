import { type ApiJsonResult, fetchApiJson } from '../transport/client.server.ts';

import type { HomeView } from './home.types.ts';

export function loadHome(request?: Request): Promise<ApiJsonResult<HomeView>> {
  return fetchApiJson<HomeView>('/home', { request, signal: request?.signal });
}
