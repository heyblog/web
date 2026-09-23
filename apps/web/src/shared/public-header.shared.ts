import type { SiteNavigationItem } from '../site-config.types.ts';

interface BrandVisibilityInput {
  readonly home: boolean;
  readonly heroBottom: number | null;
  readonly threshold: number;
}

export function resolveBrandVisibility({
  home,
  heroBottom,
  threshold,
}: BrandVisibilityInput): boolean {
  return !home || (heroBottom !== null && heroBottom <= threshold);
}

export function resolvePublicAccountEntry(hasSession: boolean): {
  readonly href: '/dashboard' | '/login';
  readonly label: '账号' | '登录';
} {
  return hasSession ? { href: '/dashboard', label: '账号' } : { href: '/login', label: '登录' };
}

export function sortNavigation(items: readonly SiteNavigationItem[]): SiteNavigationItem[] {
  return items.toSorted((left, right) => left.sort - right.sort);
}

export function navigationPath(href: string): string {
  return new URL(href, 'https://navigation.invalid').pathname.replace(/\/+$/u, '') || '/';
}

export function activeNavigationHref(
  items: readonly SiteNavigationItem[],
  pathname: string,
): string | undefined {
  const path = navigationPath(pathname);
  return items
    .filter(
      (item) =>
        path === navigationPath(item.href) ||
        (item.match === 'prefix' && path.startsWith(`${navigationPath(item.href)}/`)),
    )
    .toSorted(
      (left, right) => navigationPath(right.href).length - navigationPath(left.href).length,
    )[0]?.href;
}

// Keep the visible prefix and overflow suffix in the same sort order.
export const navigationRankClasses = [
  { direct: 'hidden sm:inline-flex', overflow: 'sm:hidden' },
  { direct: 'hidden md:inline-flex', overflow: 'md:hidden' },
  { direct: 'hidden lg:inline-flex', overflow: 'lg:hidden' },
  { direct: 'hidden lg:inline-flex', overflow: 'lg:hidden' },
] as const;
