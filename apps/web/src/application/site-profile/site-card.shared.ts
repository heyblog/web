interface SiteIdentifier {
  shortId: string;
}

export function siteDetailPath(site: SiteIdentifier): string {
  return `/site/${encodeURIComponent(site.shortId)}`;
}

export function formatSiteJoinedAt(value: string): string {
  const date = new Date(value);

  return Number.isNaN(date.getTime())
    ? '已加入目录'
    : `${date.toLocaleDateString('zh-CN', {
        year: 'numeric',
        month: 'short',
        timeZone: 'UTC',
      })}加入`;
}

export function formatSiteUpdatedAt(value: string): string {
  const date = new Date(value);

  return Number.isNaN(date.getTime())
    ? '信息更新时间未知'
    : `${date.toLocaleDateString('zh-CN', {
        year: 'numeric',
        month: 'long',
        day: 'numeric',
        timeZone: 'UTC',
      })}更新`;
}
