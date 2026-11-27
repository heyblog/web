import { z } from 'zod';

export const requestSchema = z.strictObject({
  url: z.string().url().max(8192),
  method: z.enum(['GET', 'HEAD']),
  platform: z.enum(['CF', 'EO']),
  headers: z
    .record(z.string().max(128), z.string().max(8192))
    .refine((headers) => Object.keys(headers).length <= 32),
  headerEncoding: z.enum(['utf8', 'latin1']).optional(),
  maxBytes: z
    .number()
    .int()
    .min(1)
    .max(4 << 20),
  timeoutMs: z.number().int().min(1).max(60000),
  resolvedIP: z.string().max(64),
});
export type EdgeRequest = z.infer<typeof requestSchema>;
export type Platform = EdgeRequest['platform'];
export type EdgeResult = {
  readonly status: number;
  readonly headers: Readonly<Record<string, string>>;
  readonly body: string;
  readonly node: Readonly<Record<string, string>>;
  readonly error?:
    'unsafe_target' | 'target_network' | 'target_timeout' | 'response_limit' | 'platform';
};
export type Adapter = {
  readonly platform: Platform;
  readonly token: string;
  readonly node: Readonly<Record<string, string>>;
  // A platform can decline an unsafe target before calling native Fetch.
  readonly fetch: (request: EdgeRequest, init: RequestInit) => Promise<Response | null>;
};
export const failure = (
  error: NonNullable<EdgeResult['error']>,
  node: Adapter['node'],
): EdgeResult => ({ status: 0, headers: {}, body: '', node, error });
