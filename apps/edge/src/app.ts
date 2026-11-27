import { Hono } from 'hono';
import { bodyLimit } from 'hono/body-limit';

import { execute } from './execute.ts';
import { type Adapter, requestSchema } from './protocol.ts';

async function authenticated(supplied: string, expected: string): Promise<boolean> {
  if (expected.length < 32 || supplied.length > 512) return false;
  const encode = new TextEncoder();
  const [left, right] = await Promise.all([
    crypto.subtle.digest('SHA-256', encode.encode(supplied)),
    crypto.subtle.digest('SHA-256', encode.encode(`Bearer ${expected}`)),
  ]);
  const l = new Uint8Array(left),
    r = new Uint8Array(right);
  let difference = 0;
  for (let i = 0; i < l.length; i++) difference |= (l[i] ?? 0) ^ (r[i] ?? 0);
  return difference === 0;
}
export function createApp(adapter: Adapter): Hono {
  const app = new Hono();
  app.use('*', async (c, next) => {
    c.header('Cache-Control', 'no-store');
    if (!(await authenticated(c.req.header('Authorization') ?? '', adapter.token)))
      return c.json({ error: 'unauthorized' }, 401);
    await next();
  });
  // Go's JSON encoder can expand each configured header byte into six bytes.
  // Allow the validated task headers plus proofs while retaining a hard body cap.
  app.post(
    '/v1/execute',
    bodyLimit({
      maxSize: 256 * 1024,
      onError: (c) => c.json({ error: 'request_too_large' }, 413),
    }),
    async (c) => {
      let input: unknown;
      try {
        input = await c.req.json<unknown>();
      } catch (error) {
        if (error instanceof SyntaxError) return c.json({ error: 'invalid_request' }, 400);
        throw error;
      }
      const parsed = requestSchema.safeParse(input);
      if (!parsed.success) return c.json({ error: 'invalid_request' }, 400);
      return c.json(await execute(parsed.data, adapter));
    },
  );
  app.onError(
    () =>
      new Response(JSON.stringify({ error: 'platform' }), {
        status: 500,
        headers: { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' },
      }),
  );
  return app;
}
