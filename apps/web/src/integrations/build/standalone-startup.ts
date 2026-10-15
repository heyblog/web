import { writeFile } from 'node:fs/promises';

import type { AstroConfig, AstroIntegration } from 'astro';

export async function writeStandaloneStartup(directory: URL, entry = 'entry.mjs'): Promise<void> {
  await writeFile(
    new URL('start.mjs', directory),
    `process.env.ASTRO_NODE_AUTOSTART = 'disabled';
const { startServer } = await import(${JSON.stringify(`./${entry}`)});
const runtime = startServer();
export const server = runtime.server.server;
server.requestTimeout = 30 * 60 * 1000;
server.headersTimeout = 60_000;
console.info(JSON.stringify({
  event: 'http_server_configured',
  request_timeout_ms: server.requestTimeout,
  headers_timeout_ms: server.headersTimeout,
}));
`,
  );
}

export function standaloneStartupIntegration(): AstroIntegration {
  let configuration: AstroConfig;
  return {
    name: 'heyblog-standalone-startup',
    hooks: {
      'astro:config:done': ({ config }) => {
        configuration = config;
      },
      'astro:build:done': () =>
        writeStandaloneStartup(configuration.build.server, configuration.build.serverEntry),
    },
  };
}
