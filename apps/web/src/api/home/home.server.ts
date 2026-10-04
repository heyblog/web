import { type ApiJsonResult, fetchApiJson } from '../transport/client.server.ts';

import { parseHomeView } from './home.responses.ts';
import type { HomeView } from './home.types.ts';

export async function loadHome(request?: Request): Promise<ApiJsonResult<HomeView>> {
  const result = await fetchApiJson<unknown>('/home', { request, signal: request?.signal });
  if (result.kind !== 'success') return result;
  const data = parseHomeView(result.data);
  return data ? { kind: 'success', data } : { kind: 'unavailable' };
}
