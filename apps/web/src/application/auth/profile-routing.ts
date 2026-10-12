import type { SessionUser } from '../../api/auth/auth.types.ts';

export function isAccountPage(path: string): boolean {
  return /^\/(dashboard|management)(\/|$)/u.test(path);
}

export function confirmationLocation(next: string): string {
  const destination = next.startsWith('/complete-profile') ? '/dashboard' : next;
  return `/complete-profile?next=${encodeURIComponent(destination)}`;
}

export function profileRedirect(url: URL, user: SessionUser | null): string | null {
  if (isAccountPage(url.pathname)) {
    const next = url.pathname + url.search;
    if (!user) return `/login?next=${encodeURIComponent(next)}`;
    if (user.profile_confirmation_required) return confirmationLocation(next);
  }
  return null;
}
