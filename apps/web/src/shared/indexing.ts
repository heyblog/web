const privatePrefixes = [
  '/dashboard',
  '/management',
  '/auth',
  '/api',
  '/_server-islands',
  '/site/submissions',
];
const privatePages = new Set([
  '/login',
  '/register',
  '/forgot-password',
  '/reset-password',
  '/verify-email',
  '/forbidden',
  '/site/go',
  '/random',
]);

export function isIndexablePath(pathname: string): boolean {
  const path = pathname.replace(/\/+$/, '') || '/';
  return (
    !privatePages.has(path) &&
    !privatePrefixes.some((prefix) => path === prefix || path.startsWith(`${prefix}/`))
  );
}

export function listingSeo(pathname: string, search = '') {
  const path = pathname.replace(/\/+$/, '') || '/';
  const parameters = new URLSearchParams(search);
  if (path !== '/site' && path !== '/announcements') {
    return { canonicalPath: path, indexable: true, isListing: false };
  }
  const page = parameters.get('page');
  const validPage =
    page === null || (/^[1-9]\d*$/.test(page) && Number.isSafeInteger(Number(page)));
  const onlyPage =
    [...parameters.keys()].every((key) => key === 'page') && parameters.getAll('page').length <= 1;
  return {
    canonicalPath: validPage && page !== null && page !== '1' ? `${path}?page=${page}` : path,
    indexable: validPage && onlyPage,
    isListing: true,
  };
}

export const crawlerDisallowPatterns = [
  ...privatePrefixes.flatMap((path) => [`${path}$`, `${path}/`, `${path}?`]),
  ...[...privatePages].flatMap((path) => [`${path}$`, `${path}/$`, `${path}?`, `${path}/?`]),
];
