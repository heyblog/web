import { createHash, createHmac } from 'node:crypto';
import { readFile } from 'node:fs/promises';

import { z } from 'zod';

const config = z
  .object({
    TENCENT_SECRET_ID: z.string().min(1),
    TENCENT_SECRET_KEY: z.string().min(1),
    EDGEONE_ZONE_ID: z.string().min(1),
    EDGEONE_FUNCTION_ID: z.string().min(1),
  })
  .safeParse(process.env);
if (!config.success)
  throw new Error(
    'Set TENCENT_SECRET_ID, TENCENT_SECRET_KEY, EDGEONE_ZONE_ID and EDGEONE_FUNCTION_ID.',
  );
const env = config.data;
const body = JSON.stringify({
  ZoneId: env.EDGEONE_ZONE_ID,
  FunctionId: env.EDGEONE_FUNCTION_ID,
  Content: await readFile('dist/edgeone.js', 'utf8'),
});
const timestamp = Math.floor(Date.now() / 1000);
const date = new Date(timestamp * 1000).toISOString().slice(0, 10);
const digest = (value) => createHash('sha256').update(value).digest('hex');
const hmac = (key, value) => createHmac('sha256', key).update(value).digest();
const canonical = `POST\n/\n\ncontent-type:application/json; charset=utf-8\nhost:teo.tencentcloudapi.com\n\ncontent-type;host\n${digest(body)}`;
const scope = `${date}/teo/tc3_request`;
const stringToSign = `TC3-HMAC-SHA256\n${timestamp}\n${scope}\n${digest(canonical)}`;
const key = hmac(hmac(hmac(`TC3${env.TENCENT_SECRET_KEY}`, date), 'teo'), 'tc3_request');
const signature = createHmac('sha256', key).update(stringToSign).digest('hex');
const response = await fetch('https://teo.tencentcloudapi.com/', {
  method: 'POST',
  headers: {
    Authorization: `TC3-HMAC-SHA256 Credential=${env.TENCENT_SECRET_ID}/${scope}, SignedHeaders=content-type;host, Signature=${signature}`,
    'Content-Type': 'application/json; charset=utf-8',
    'X-TC-Action': 'ModifyFunction',
    'X-TC-Version': '2022-09-01',
    'X-TC-Timestamp': String(timestamp),
  },
  body,
  signal: AbortSignal.timeout(60000),
  redirect: 'error',
});
const result = z
  .object({
    Response: z.object({
      Error: z.object({ Code: z.string() }).optional(),
      RequestId: z.string().optional(),
    }),
  })
  .safeParse(await response.json());
if (!response.ok || !result.success || result.data.Response.Error)
  throw new Error(
    `EdgeOne deployment rejected (HTTP ${response.status}); inspect the provider dashboard.`,
  );
console.log(
  `Updated EdgeOne function ${env.EDGEONE_FUNCTION_ID}. Verify its encrypted token, HTTPS trigger and mainland execution before enabling tasks.`,
);
