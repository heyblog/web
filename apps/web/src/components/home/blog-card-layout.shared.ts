import type { HomeSiteCard } from '@/application/home/home.shared';

export type BlogCardTagTone = 'warning' | 'primary' | 'secondary';

export type BlogCardUpdateTone = 'emerald' | 'amber' | 'blue' | 'stone';

export type BlogCardContentUpdate = {
  readonly label: string;
  readonly tone: BlogCardUpdateTone;
};

export type BlogCardPlannedFields = {
  readonly visitCount?: number;
  readonly articleCount?: number;
  readonly contentUpdated?: BlogCardContentUpdate;
  readonly feedback?: boolean;
};

export interface BlogCardTag {
  key: string;
  label: string;
  tone: BlogCardTagTone;
}

export interface TagLayoutMeasurements {
  containerWidth: number;
  tagWidths: readonly number[];
  counterWidths: readonly number[];
  gap: number;
  maxRows?: number;
}

export type BlogCardSourceRect = {
  readonly left: number;
  readonly top: number;
  readonly width: number;
  readonly height: number;
};

export type BlogCardViewport = {
  readonly width: number;
  readonly height: number;
};

export type AnchoredDialogMeasurements = {
  readonly source: BlogCardSourceRect;
  readonly dialogHeight: number;
  readonly viewport: BlogCardViewport;
};

export type AnchoredDialogLayout = {
  readonly left: number;
  readonly top: number;
  readonly width: number;
  readonly maxHeight: number;
};

const VIEWPORT_INSET = 16;

export function resolveAnchoredDialogLayout({
  source,
  dialogHeight,
  viewport,
}: AnchoredDialogMeasurements): AnchoredDialogLayout {
  const maxWidth = Math.max(0, viewport.width - VIEWPORT_INSET * 2);
  const maxHeight = Math.max(0, viewport.height - VIEWPORT_INSET * 2);
  const width = Math.min(source.width, maxWidth);
  const height = Math.min(dialogHeight, maxHeight);

  return {
    left: Math.min(Math.max(VIEWPORT_INSET, source.left), viewport.width - VIEWPORT_INSET - width),
    top: Math.min(Math.max(VIEWPORT_INSET, source.top), viewport.height - VIEWPORT_INSET - height),
    width,
    maxHeight,
  };
}

export function createBlogCardTags(site: Pick<HomeSiteCard, 'topics' | 'warnings'>): BlogCardTag[] {
  const primaryTopic = site.topics.find((topic) => topic.role === 'PRIMARY');
  const secondaryTopics = site.topics.filter((topic) => topic.role === 'SECONDARY');

  return [
    ...site.warnings.map((warning) => ({
      key: `warning:${warning.slug}`,
      label: warning.name,
      tone: 'warning' as const,
    })),
    primaryTopic
      ? {
          key: `primary:${primaryTopic.slug}`,
          label: primaryTopic.name,
          tone: 'primary' as const,
        }
      : {
          key: 'primary:uncategorized',
          label: '未分类',
          tone: 'secondary' as const,
        },
    ...secondaryTopics.map((topic) => ({
      key: `secondary:${topic.slug}`,
      label: topic.name,
      tone: 'secondary' as const,
    })),
  ];
}

export function resolveVisibleTagCount({
  containerWidth,
  tagWidths,
  counterWidths,
  gap,
  maxRows = 2,
}: TagLayoutMeasurements): number {
  if (containerWidth <= 0 || tagWidths.length === 0) {
    return tagWidths.length;
  }
  if (fitsWithinRows(tagWidths, containerWidth, gap, maxRows)) {
    return tagWidths.length;
  }

  for (let visibleCount = tagWidths.length - 1; visibleCount >= 0; visibleCount -= 1) {
    const hiddenCount = tagWidths.length - visibleCount;
    const counterWidth = counterWidths[hiddenCount];
    if (
      counterWidth !== undefined &&
      fitsWithinRows(
        [...tagWidths.slice(0, visibleCount), counterWidth],
        containerWidth,
        gap,
        maxRows,
      )
    ) {
      return visibleCount;
    }
  }

  return 0;
}

function fitsWithinRows(
  widths: readonly number[],
  containerWidth: number,
  gap: number,
  maxRows: number,
): boolean {
  if (maxRows <= 0) return false;

  let row = 1;
  let usedWidth = 0;
  for (const width of widths) {
    if (width > containerWidth) return false;

    const nextWidth = usedWidth === 0 ? width : usedWidth + gap + width;
    if (nextWidth <= containerWidth + 0.5) {
      usedWidth = nextWidth;
      continue;
    }

    row += 1;
    if (row > maxRows) return false;
    usedWidth = width;
  }

  return true;
}
