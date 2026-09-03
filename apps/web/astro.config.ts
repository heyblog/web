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
        "connect-src 'self'",
        "font-src 'self' data:",
        "form-action 'self'",
        "frame-ancestors 'none'",
        "frame-src 'none'",
        "img-src 'self' data:",
        "manifest-src 'self'",
        "object-src 'none'",
        'upgrade-insecure-requests',
      ],
    },
  },
  vite: {
    plugins: [tailwindcss()],
  },
  integrations: [buildMetadataIntegration(), svelte(), mdx(), sitemap()],
});
