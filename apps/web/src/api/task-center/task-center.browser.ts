import {
  type ManualInput,
  parseRequestConfig,
  type RequestConfigInput,
} from './configuration.types.ts';
import {
  parseAttempt,
  parseOverview,
  parseStatistics,
  parseTaskEvent,
  parseWorker,
} from './observability.types.ts';
import { choice, list, ProtocolError, record } from './parse.ts';
import { parseEvent, parseKey, parsePolicy, parseProof, type Policy } from './signing.types.ts';
import {
  type Control,
  parseAccepted,
  parseBatchItem,
  parsePage,
  parseRun,
  parseSchedule,
  parseTarget,
  type RetrySelection,
  type Run,
  runStates,
  type ScheduleInput,
  type TaskQuery,
  taskQuery,
} from './tasks.types.ts';
export class TaskTransportError extends Error {
  readonly status: number;
  readonly code: string;
  readonly currentState?: Run['state'];
  constructor(status: number, code: string, currentState?: Run['state']) {
    super(code);
    this.status = status;
    this.code = code;
    this.currentState = currentState;
  }
}
async function request<T>(
  path: string,
  parse: (value: unknown) => T,
  body?: object,
  method = 'GET',
  key?: string,
): Promise<T> {
  let response: Response;
  try {
    response = await fetch(`/api/web/${path}`, {
      method,
      credentials: 'same-origin',
      cache: 'no-store',
      headers: {
        Accept: 'application/json',
        ...(body ? { 'Content-Type': 'application/json' } : {}),
        ...(method !== 'GET' ? { 'Idempotency-Key': key ?? crypto.randomUUID() } : {}),
      },
      body: body ? JSON.stringify(body) : undefined,
      signal: AbortSignal.timeout(15000),
    });
  } catch (error) {
    if (error instanceof Error) throw new TaskTransportError(0, 'network_error');
    throw error;
  }
  let value: unknown;
  try {
    value = await response.json();
  } catch (error) {
    if (error instanceof Error) throw new TaskTransportError(response.status, 'invalid_response');
    throw error;
  }
  if (!response.ok) {
    const problem = record(value);
    throw new TaskTransportError(
      response.status,
      typeof problem.code === 'string' ? problem.code : 'request_failed',
      problem.current_state === undefined ? undefined : choice(problem.current_state, runStates),
    );
  }
  try {
    return parse(value);
  } catch (error) {
    if (error instanceof ProtocolError) throw new TaskTransportError(0, 'invalid_response');
    throw error;
  }
}
const tasks = 'management/tasks',
  signing = 'management/request-signing';
export const taskAPI = {
  schedules: () => request(`${tasks}/schedules`, (v) => list(v, parseSchedule)),
  save: (value: ScheduleInput, id?: string) =>
    request(
      `${tasks}/schedules${id ? `/${encodeURIComponent(id)}` : ''}`,
      parseSchedule,
      {
        name: value.name,
        kind: value.kind,
        enabled: value.enabled,
        archived: value.archived,
        configuration: value.configuration,
        revision: value.revision,
      },
      id ? 'PUT' : 'POST',
    ),
  run: (id: string) =>
    request(`${tasks}/schedules/${encodeURIComponent(id)}/run`, parseAccepted, {}, 'POST'),
  runs: (query: TaskQuery = {}) =>
    request(`${tasks}/runs${taskQuery(query)}`, (v) => parsePage(v, parseRun)),
  detail: (id: string) => request(`${tasks}/runs/${encodeURIComponent(id)}`, parseRun),
  targets: (id: string, query: TaskQuery = {}) =>
    request(`${tasks}/runs/${encodeURIComponent(id)}/targets${taskQuery(query)}`, (v) =>
      parsePage(v, parseTarget),
    ),
  attempts: (id: string, query: TaskQuery = {}) =>
    request(`${tasks}/runs/${encodeURIComponent(id)}/attempts${taskQuery(query)}`, (v) =>
      parsePage(v, parseAttempt),
    ),
  events: (id: string, query: TaskQuery = {}) =>
    request(`${tasks}/runs/${encodeURIComponent(id)}/events${taskQuery(query)}`, (v) =>
      parsePage(v, parseTaskEvent),
    ),
  control: (id: string, action: Control) =>
    request(`${tasks}/runs/${encodeURIComponent(id)}/${action}`, parseRun, {}, 'POST'),
  create: (value: ManualInput, key?: string) =>
    request(`${tasks}/runs`, parseAccepted, value, 'POST', key),
  retry: (id: string, selection: RetrySelection) =>
    request(
      `${tasks}/runs/${encodeURIComponent(id)}/${selection === 'all' ? 'rerun' : 'retry'}`,
      parseAccepted,
      { selection },
      'POST',
    ),
  batch: (ids: readonly string[], action: Control) =>
    request(`${tasks}/runs/batch`, (v) => list(v, parseBatchItem), { ids, action }, 'POST'),
  cleanup: (id: string, confirmation: string) =>
    request(`${tasks}/runs/${encodeURIComponent(id)}/cleanup`, parseRun, { confirmation }, 'POST'),
  configs: () => request(`${tasks}/request-configs`, (v) => list(v, parseRequestConfig)),
  bindVerification: (id: string) =>
    request(
      `${tasks}/request-configs/${encodeURIComponent(id)}/verification-default`,
      parseAccepted,
      {},
      'POST',
    ),
  saveConfig: (value: RequestConfigInput, id?: string) =>
    request(
      `${tasks}/request-configs${id ? `/${encodeURIComponent(id)}` : ''}`,
      parseRequestConfig,
      {
        name: value.name,
        purpose: value.purpose,
        enabled: value.enabled,
        archived: value.archived,
        revision: value.revision,
        parameters: value.parameters,
      },
      id ? 'PUT' : 'POST',
    ),
  overview: () => request(`${tasks}/overview`, parseOverview),
  workers: () => request(`${tasks}/workers`, (v) => list(v, parseWorker)),
  worker: (id: string, action: 'drain' | 'resume') =>
    request(`${tasks}/workers/${encodeURIComponent(id)}/${action}`, parseAccepted, {}, 'POST'),
  stats: (query: TaskQuery = {}) =>
    request(`${tasks}/stats${taskQuery(query)}`, (v) => list(v, parseStatistics)),
  export: async (id: string, format: 'json' | 'csv', query: TaskQuery = {}) => {
    const params = new URLSearchParams(taskQuery(query).slice(1));
    params.set('format', format);
    let response: Response;
    try {
      response = await fetch(`/api/web/${tasks}/runs/${encodeURIComponent(id)}/export?${params}`, {
        credentials: 'same-origin',
        cache: 'no-store',
        headers: { Accept: 'application/json' },
        signal: AbortSignal.timeout(30000),
      });
    } catch (error) {
      if (error instanceof Error) throw new TaskTransportError(0, 'network_error');
      throw error;
    }
    if (!response.ok) throw new TaskTransportError(response.status, 'export_failed');
    return response.blob();
  },
};
export const signingAPI = {
  policy: () => request(`${signing}/policy`, parsePolicy),
  savePolicy: (value: Policy) => request(`${signing}/policy`, parsePolicy, value, 'PUT'),
  keys: () => request(`${signing}/keys`, (v) => list(v, parseKey)),
  generate: (platform: 'CF' | 'EO') =>
    request(`${signing}/keys`, parseAccepted, { platform }, 'POST'),
  transition: (id: string, action: 'publish' | 'activate' | 'revoke') =>
    request(`${signing}/keys/${encodeURIComponent(id)}/${action}`, parseAccepted, {}, 'POST'),
  events: () => request(`${signing}/events`, (v) => list(v, parseEvent)),
  publicKeys: () => request('public/request-signing/keys', (v) => list(v, parseKey)),
  verify: (token: string, observed?: { readonly method: string; readonly url: string }) =>
    request('public/request-signing/verify', parseProof, { token, ...observed }, 'POST'),
};
