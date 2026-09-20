// @ts-check
import mdx from '@astrojs/mdx';
import node from '@astrojs/node';
import sitemap from '@astrojs/sitemap';
import svelte from '@astrojs/svelte';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'astro/config';

import { buildMetadataIntegration } from './src/shared/integrations/build-metadata';
import { siteConfig } from './src/site.config';

// https://astro.build/config
export default defineConfig({
  adapter: node({
    mode: 'standalone',
  }),
  server: {
    host: true,
    port: 10101,
  },
  output: 'server',
  site: siteConfig.url,
  markdown: {
    syntaxHighlight: 'prism',
  },
  prefetch: {
    prefetchAll: true,
    defaultStrategy: 'hover',
  },
  security: {
    allowedDomains: [{ protocol: 'https', hostname: 'www.heyblog.net', port: '443' }],
    csp: {
      directives: [
        "default-src 'self'",
        "base-uri 'none'",
        "connect-src 'self' https://cloudflareinsights.com https://*.google-analytics.com https://*.analytics.google.com https://*.googletagmanager.com",
        "font-src 'self' data:",
        "form-action 'self'",
        "frame-ancestors 'none'",
        "frame-src 'none'",
        "img-src 'self' data: https://*.google-analytics.com https://*.googletagmanager.com",
        "manifest-src 'self'",
        "object-src 'none'",
        'upgrade-insecure-requests',
      ],
      scriptDirective: {
        resources: [
          "'self'",
          'https://static.cloudflareinsights.com/beacon.min.js',
          'https://www.googletagmanager.com/gtag/js',
        ],
      },
    },
  },
  vite: {
    plugins: [tailwindcss()],
    ssr: {
      noExternal: ['@resvg/resvg-wasm', 'qrcode', 'pngjs'],
      optimizeDeps: { include: ['qrcode'] },
    },
  },
  integrations: [buildMetadataIntegration(), svelte(), mdx(), sitemap()],
});
