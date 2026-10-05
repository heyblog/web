import { isRecord, isRevision } from './taxonomy.types.ts';

export interface AISettings {
  readonly model_id: string;
  readonly revision: string;
  readonly configured: boolean;
}

export interface AIModel {
  readonly id: string;
  readonly name: string;
  readonly status: string;
}

export interface AIModels {
  readonly models: readonly AIModel[];
  readonly configured: boolean;
}

export function parseSettings(value: unknown): AISettings | null {
  return isRecord(value) &&
    typeof value.model_id === 'string' &&
    isRevision(value.revision) &&
    typeof value.configured === 'boolean'
    ? { model_id: value.model_id, revision: value.revision, configured: value.configured }
    : null;
}

export function parseModels(value: unknown): AIModels | null {
  const guard = (item: unknown): item is AIModel =>
    isRecord(item) &&
    typeof item.id === 'string' &&
    typeof item.name === 'string' &&
    typeof item.status === 'string';
  return isRecord(value) &&
    Array.isArray(value.models) &&
    value.models.every(guard) &&
    typeof value.configured === 'boolean'
    ? { models: value.models, configured: value.configured }
    : null;
}
