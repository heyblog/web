import { randomBytes } from 'node:crypto';

import { parseRequestID } from './request-id.ts';

export function resolveRequestID(request: Request): string {
  return parseRequestID(request.headers.get('X-Request-ID')) ?? randomBytes(16).toString('hex');
}
