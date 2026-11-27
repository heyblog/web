import { build } from 'esbuild';

await Promise.all([
  build({
    entryPoints: ['src/cloudflare.ts'],
    bundle: true,
    format: 'esm',
    platform: 'browser',
    target: 'es2022',
    outfile: 'dist/cloudflare.mjs',
  }),
  build({
    entryPoints: ['src/edgeone.ts'],
    bundle: true,
    format: 'iife',
    platform: 'browser',
    target: 'es2022',
    outfile: 'dist/edgeone.js',
  }),
]);
