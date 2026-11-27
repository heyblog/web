import { readFile } from 'node:fs/promises';

import { z } from 'zod';

const config = z
  .object({
    CLOUDFLARE_ACCOUNT_ID: z.string().regex(/^[a-zA-Z0-9]+$/u),
    CLOUDFLARE_API_TOKEN: z.string().min(1),
    CLOUDFLARE_WORKER_NAME: z.string().regex(/^[a-z0-9-]+$/u),
    EDGE_SERVICE_TOKEN: z.string().min(32),
  })
  .safeParse(process.env);
if (!config.success)
  throw new Error(
    'Set CLOUDFLARE_ACCOUNT_ID, CLOUDFLARE_API_TOKEN, CLOUDFLARE_WORKER_NAME and EDGE_SERVICE_TOKEN.',
  );
const env = config.data;
const metadata = z
  .object({
    main_module: z.string(),
    compatibility_date: z.string(),
    compatibility_flags: z.array(z.string()),
    bindings: z.array(z.unknown()).length(0),
  })
  .parse(JSON.parse(await readFile('cloudflare.json', 'utf8')));
if (!metadata.compatibility_flags.includes('global_fetch_strictly_public'))
  throw new Error('Cloudflare public-only egress must be enabled.');
const body = new FormData();
body.set(
  'metadata',
  new Blob(
    [
      JSON.stringify({
        ...metadata,
        bindings: [
          { type: 'secret_text', name: 'EDGE_SERVICE_TOKEN', text: env.EDGE_SERVICE_TOKEN },
        ],
      }),
    ],
    { type: 'application/json' },
  ),
);
body.set(
  'cloudflare.mjs',
  new Blob([await readFile('dist/cloudflare.mjs')], { type: 'application/javascript+module' }),
  'cloudflare.mjs',
);
const response = await fetch(
  `https://api.cloudflare.com/client/v4/accounts/${env.CLOUDFLARE_ACCOUNT_ID}/workers/scripts/${env.CLOUDFLARE_WORKER_NAME}`,
  {
    method: 'PUT',
    headers: { Authorization: `Bearer ${env.CLOUDFLARE_API_TOKEN}` },
    body,
    signal: AbortSignal.timeout(60000),
    redirect: 'error',
  },
);
const result = z.object({ success: z.boolean() }).safeParse(await response.json());
if (!response.ok || !result.success || !result.data.success)
  throw new Error(
    `Cloudflare deployment rejected (HTTP ${response.status}); inspect the provider dashboard.`,
  );
console.log(
  `Updated Cloudflare worker ${env.CLOUDFLARE_WORKER_NAME}. Verify its Custom Domain before enabling tasks.`,
);
