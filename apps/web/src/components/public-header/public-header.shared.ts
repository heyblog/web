import type { IconArrowRight } from '@tabler/icons-svelte-runes';

export type PublicPrimaryNavKey = 'directory' | 'subscription' | 'random' | 'stats';
export type PublicProjectNavKey = 'docs' | 'blog' | 'contribute' | 'about';

export interface PublicHeaderLink {
  navKey: PublicPrimaryNavKey | PublicProjectNavKey;
  label: string;
  href: string;
  icon: typeof IconArrowRight;
}

export interface PublicHeaderActiveStateInput {
  currentPath: string;
  activePrimaryKey?: PublicPrimaryNavKey | null;
  activeProjectKey?: PublicProjectNavKey | null;
}

export interface PublicHeaderActiveState {
  activePrimaryKey: PublicPrimaryNavKey | null;
  activeProjectKey: PublicProjectNavKey | null;
  projectActive: boolean;
}

function normalizePath(value: string): string {
  if (value.length > 1 && value.endsWith('/')) {
    return value.slice(0, -1);
  }

  return value || '/';
}

function resolvePrimaryKey(pathname: string): PublicPrimaryNavKey | null {
  if (pathname === '/site') {
    return 'directory';
  }

  if (pathname === '/site/go') {
    return 'random';
  }

  if (pathname === '/site/subscription') {
    return 'subscription';
  }

  if (pathname === '/site/stats') {
    return 'stats';
  }

  return null;
}

function resolveProjectKey(pathname: string): PublicProjectNavKey | null {
  if (pathname === '/docs' || pathname.startsWith('/docs/')) {
    return 'docs';
  }

  if (pathname === '/blog' || pathname.startsWith('/blog/')) {
    return 'blog';
  }

  if (pathname === '/contribute') {
    return 'contribute';
  }

  if (pathname === '/about') {
    return 'about';
  }

  return null;
}

export function resolvePublicHeaderActiveState(
  input: PublicHeaderActiveStateInput,
): PublicHeaderActiveState {
  const currentPath = normalizePath(input.currentPath);
  const activePrimaryKey = input.activePrimaryKey ?? resolvePrimaryKey(currentPath);
  const activeProjectKey = input.activeProjectKey ?? resolveProjectKey(currentPath);

  return {
    activePrimaryKey,
    activeProjectKey,
    projectActive: activeProjectKey !== null,
  };
}
