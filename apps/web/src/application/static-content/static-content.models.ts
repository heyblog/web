import type { MemberStatusKey } from '../../shared/content/member-status.ts';
import { memberStatusOrder } from '../../shared/content/member-status.ts';

export interface ContentEditorLink {
  id: string;
  label: string;
  href: string;
}

export interface BlogSummary {
  id: string;
  title: string;
  description: string;
  createTime: Date;
  category: string;
  editors: ContentEditorLink[];
  tags: string[];
  sort?: number;
  top: boolean;
}

export interface MemberSummary {
  id: string;
  displayName: string;
  homepageUrl: string;
  githubUrl: string;
  title: string;
  description: string;
  tags: string[];
  status: MemberStatusKey;
  joinTime?: string;
  leaveTime?: string;
  sort?: number;
}

export interface ContentHeading {
  depth: number;
  slug: string;
  text: string;
}

const idCollator = new Intl.Collator('zh-CN', {
  numeric: true,
  sensitivity: 'base',
});

const optionalSortValue = (value: number | undefined): number => value ?? Number.MAX_SAFE_INTEGER;

export function compareBlogSummaries(left: BlogSummary, right: BlogSummary): number {
  if (left.top !== right.top) {
    return left.top ? -1 : 1;
  }

  const explicitSort = optionalSortValue(left.sort) - optionalSortValue(right.sort);

  if (explicitSort !== 0) {
    return explicitSort;
  }

  const dateOrder = right.createTime.getTime() - left.createTime.getTime();

  return dateOrder || idCollator.compare(left.id, right.id);
}

export function sortBlogSummaries(items: BlogSummary[]): BlogSummary[] {
  return items.toSorted(compareBlogSummaries);
}

export function compareMemberSummaries(left: MemberSummary, right: MemberSummary): number {
  const statusOrder = memberStatusOrder[left.status] - memberStatusOrder[right.status];

  if (statusOrder !== 0) {
    return statusOrder;
  }

  const explicitSort = optionalSortValue(left.sort) - optionalSortValue(right.sort);

  return explicitSort || idCollator.compare(left.id, right.id);
}

export function sortMemberSummaries(items: MemberSummary[]): MemberSummary[] {
  return items.toSorted(compareMemberSummaries);
}

export function formatContentDate(value: Date | string | undefined): string {
  if (!value) {
    return '';
  }

  const date = value instanceof Date ? value : new Date(value);

  if (Number.isNaN(date.getTime())) {
    return '';
  }

  return new Intl.DateTimeFormat('zh-CN', { dateStyle: 'long' }).format(date);
}

export function getDocRoutePath(entryId: string): string {
  return entryId === 'index' ? '/docs' : `/docs/${entryId}`;
}

const normalizeHeadingText = (value: string): string =>
  value.trim().replaceAll(/\s+/gu, ' ').toLocaleLowerCase('zh-CN');

export function shouldSuppressLeadingContentTitle(
  title: string,
  headings: ContentHeading[],
): boolean {
  const levelOneIndexes = headings.flatMap((heading, index) =>
    heading.depth === 1 ? [index] : [],
  );

  if (levelOneIndexes.length === 0) {
    return false;
  }

  const firstHeading = headings[0];

  if (
    levelOneIndexes.length !== 1 ||
    levelOneIndexes[0] !== 0 ||
    !firstHeading ||
    normalizeHeadingText(firstHeading.text) !== normalizeHeadingText(title)
  ) {
    throw new Error(`Content entry "${title}" must use its frontmatter title as the only H1.`);
  }

  return true;
}
