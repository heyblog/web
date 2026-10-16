import { readFileSync } from 'node:fs';
import { readFile } from 'node:fs/promises';
import { registerHooks, stripTypeScriptTypes } from 'node:module';

import { compileModule } from 'svelte/compiler';

import { createBackupTransport } from '../src/api/database-backup/database-backup.browser.ts';
import type { CurrentBackupUpload } from '../src/api/database-backup/database-backup.uploads.ts';

import { uploadsFixture } from './database-backup.fixture.ts';

const sourceURL = new URL(
  '../src/application/database-backup/database-backup.controller.svelte.ts',
  import.meta.url,
);
const source = stripTypeScriptTypes(await readFile(sourceURL, 'utf8'));
const compiled = compileModule(source, { filename: sourceURL.pathname, generate: 'client' }).js
  .code;
const hooks = registerHooks({
  load(url, context, next) {
    if (url === sourceURL.href) return { format: 'module', shortCircuit: true, source: compiled };
    if (url.endsWith('/database-backup.upload-controller.svelte.ts')) {
      const source = stripTypeScriptTypes(readFileSync(new URL(url), 'utf8'));
      return {
        format: 'module',
        shortCircuit: true,
        source: compileModule(source, { filename: new URL(url).pathname, generate: 'client' }).js
          .code,
      };
    }
    return next(url, context);
  },
});
const module =
  await import('../src/application/database-backup/database-backup.controller.svelte.ts');
hooks.deregister();

export function createTestBackupController(
  transport = createBackupTransport(),
  initial: CurrentBackupUpload | null = uploadsFixture,
) {
  return module.createBackupController(transport, initial);
}
