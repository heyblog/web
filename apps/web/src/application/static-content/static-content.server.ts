import {
  type MemberReference,
  readBlogEntries,
  readContentEditors,
  readMemberEntries,
} from '../../integrations/content/content.server.ts';

import type { BlogSummary, ContentEditorLink, MemberSummary } from './static-content.models.ts';
import { sortBlogSummaries, sortMemberSummaries } from './static-content.models.ts';

export async function resolveContentEditors(
  references: MemberReference[],
): Promise<ContentEditorLink[]> {
  const editors = await readContentEditors(references);

  return editors.map((editor) => ({
    id: editor.id,
    label: editor.data.nickname || editor.id,
    href: `/members#${encodeURIComponent(editor.id)}`,
  }));
}

export async function readBlogSummaries(): Promise<BlogSummary[]> {
  const entries = await readBlogEntries();
  const summaries = await Promise.all(
    entries.map(async (entry) => ({
      id: entry.id,
      title: entry.data.title,
      description: entry.data.description,
      createTime: entry.data.create_time,
      category: entry.data.category,
      editors: await resolveContentEditors(entry.data.editors),
      tags: entry.data.tags,
      sort: entry.data.sort,
      top: entry.data.top,
    })),
  );

  return sortBlogSummaries(summaries);
}

export async function readMemberDirectory(): Promise<{
  current: MemberSummary[];
  alumni: MemberSummary[];
}> {
  const entries = await readMemberEntries();
  const members = sortMemberSummaries(
    entries.map((entry) => ({
      id: entry.id,
      displayName: entry.data.nickname || entry.id,
      homepageUrl: entry.data.url || `https://github.com/${entry.id}`,
      githubUrl: `https://github.com/${entry.id}`,
      title: entry.data.title,
      description: entry.data.description,
      tags: entry.data.tags,
      status: entry.data.status,
      joinTime: entry.data.join_time,
      leaveTime: entry.data.leave_time,
      sort: entry.data.sort,
    })),
  );

  return {
    current: members.filter((member) => member.status !== 'ALUMNI'),
    alumni: members.filter((member) => member.status === 'ALUMNI'),
  };
}
