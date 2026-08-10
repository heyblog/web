// @ts-check
import mdx from '@astrojs/mdx';
import node from '@astrojs/node';
import partytown from '@astrojs/partytown';
import sitemap from '@astrojs/sitemap';
import svelte from '@astrojs/svelte';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'astro/config';

import { siteConfig } from './src/site.config';

// https://astro.build/config
export default defineConfig({
  adapter: node({
    mode: 'standalone',
  }),
  output: 'server',
  site: siteConfig.url,
  vite: {
    plugins: [tailwindcss()],
  },
  integrations: [svelte(), mdx(), sitemap(), partytown()],
});
