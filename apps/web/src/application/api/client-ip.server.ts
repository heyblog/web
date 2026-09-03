import { isIP } from 'node:net';

export function forwardClientAddress(request: Request, upstreamHeaders: Headers): void {
  const address = request.headers.get('X-Real-IP')?.trim() ?? '';
  if (isIP(address) === 0) return;
  upstreamHeaders.set('X-Real-IP', address);
  upstreamHeaders.set('X-Forwarded-For', address);
}
