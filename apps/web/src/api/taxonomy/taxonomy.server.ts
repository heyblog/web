import { requestAuthAPI } from '../auth/auth.server.ts';

import { parseSettings } from './system-settings.types.ts';
import { parseTaxonomy } from './taxonomy.types.ts';

export async function readTaxonomyPage(request: Request) {
  const response = await requestAuthAPI(request, '/management/taxonomy/tags');
  if (!response.ok) throw new Error('taxonomy unavailable');
  const value: unknown = await response.json();
  const data = parseTaxonomy(value);
  if (!data) throw new Error('invalid taxonomy response');
  return data;
}

export async function readSettingsPage(request: Request) {
  const response = await requestAuthAPI(request, '/management/system-settings');
  if (!response.ok) throw new Error('system settings unavailable');
  const value: unknown = await response.json();
  const data = parseSettings(value);
  if (!data) throw new Error('invalid settings response');
  return data;
}
