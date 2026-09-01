import type { BlogSummary } from '@/application/static-content/static-content.models';

export const HOME_PROJECT_BLOG_LIMIT = 3;

export function selectHomeProjectBlogs(blogs: BlogSummary[]): BlogSummary[] {
  return blogs.slice(0, HOME_PROJECT_BLOG_LIMIT);
}
