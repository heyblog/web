export const LEGACY_DOMAIN_NOTICE_DURATION_MS = 10_000;

const LEGACY_SOURCE_PARAMETERS = ['from', 'via'] as const;

const legacyDomainAliases = new Map<string, LegacyDomain>([
  ['zhblogs.net', 'zhblogs.net'],
  ['www.zhblogs.net', 'zhblogs.net'],
  ['zhblogs.cn', 'zhblogs.cn'],
  ['www.zhblogs.cn', 'zhblogs.cn'],
  ['zhblogs.org', 'zhblogs.org'],
  ['www.zhblogs.org', 'zhblogs.org'],
  ['zhblogs.ohyee.cc', 'zhblogs.ohyee.cc'],
  ['www.zhblogs.ohyee.cc', 'zhblogs.ohyee.cc'],
]);

export type LegacyDomain = 'zhblogs.net' | 'zhblogs.cn' | 'zhblogs.org' | 'zhblogs.ohyee.cc';

export interface LegacyDomainNoticeTarget {
  readonly hostname: string;
  readonly name: string;
}

export interface LegacyDomainNotice {
  readonly durationMs: number;
  readonly eyebrow: string;
  readonly id: string;
  readonly message: string;
  readonly sourceDomain: LegacyDomain;
  readonly targetHostname: string;
  readonly title: string;
  readonly tone: 'warning';
}

const parseUrl = (input: string): URL | null => {
  if (URL.canParse(input)) {
    return new URL(input);
  }

  const absoluteInput = `https://${input}`;
  return URL.canParse(absoluteInput) ? new URL(absoluteInput) : null;
};

export const normalizeLegacyDomain = (input: string | null): LegacyDomain | null => {
  const candidate = input?.trim();

  if (!candidate) {
    return null;
  }

  const hostname = parseUrl(candidate)?.hostname.toLowerCase();
  return hostname ? (legacyDomainAliases.get(hostname) ?? null) : null;
};

const buildLegacyDomainNotice = (
  sourceDomain: LegacyDomain,
  target: LegacyDomainNoticeTarget,
): LegacyDomainNotice =>
  ({
    durationMs: LEGACY_DOMAIN_NOTICE_DURATION_MS,
    eyebrow: '域名迁移提醒',
    id: `legacy-domain:${sourceDomain}`,
    message: `您通过旧域名 ${sourceDomain} 进入本站。请将书签、订阅地址和常用入口更新为 ${target.hostname}。`,
    sourceDomain,
    targetHostname: target.hostname,
    title: `zhblogs 已迁移至 ${target.name}`,
    tone: 'warning',
  }) satisfies LegacyDomainNotice;

export const getLegacyDomainNoticeFromUrl = (
  input: string | URL,
  target: LegacyDomainNoticeTarget,
): LegacyDomainNotice | null => {
  const url = input instanceof URL ? input : parseUrl(input);

  if (!url) {
    return null;
  }

  const sourceDomain = LEGACY_SOURCE_PARAMETERS.map((parameter) =>
    normalizeLegacyDomain(url.searchParams.get(parameter)),
  ).find((domain) => domain !== null);

  return sourceDomain ? buildLegacyDomainNotice(sourceDomain, target) : null;
};

export const getUrlWithoutLegacySource = (input: URL): string => {
  const cleanUrl = new URL(input);

  for (const parameter of LEGACY_SOURCE_PARAMETERS) {
    cleanUrl.searchParams.delete(parameter);
  }

  return `${cleanUrl.pathname}${cleanUrl.search}${cleanUrl.hash}`;
};
