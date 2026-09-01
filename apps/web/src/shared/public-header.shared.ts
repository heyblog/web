interface BrandVisibilityInput {
  home: boolean;
  heroBottom: number | null;
  threshold: number;
}

export function resolvePublicAccountEntry(hasSession: boolean): {
  readonly href: '/dashboard' | '/login';
  readonly label: '账号' | '登录';
} {
  return hasSession ? { href: '/dashboard', label: '账号' } : { href: '/login', label: '登录' };
}

export function resolveBrandVisibility({
  home,
  heroBottom,
  threshold,
}: BrandVisibilityInput): boolean {
  if (!home) {
    return true;
  }

  return heroBottom !== null && heroBottom <= threshold;
}
