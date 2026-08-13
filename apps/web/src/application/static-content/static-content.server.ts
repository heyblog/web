import type { CollectionEntry } from 'astro:content';
import { getCollection, getEntries, getEntry } from 'astro:content';

import type { BlogSummary, ContentEditorLink, MemberSummary } from './static-content.models';
import { sortBlogSummaries, sortMemberSummaries } from './static-content.models';

type MemberReference =
  | CollectionEntry<'blogs'>['data']['editors'][number]
  | CollectionEntry<'docs'>['data']['editors'][number];

export async function resolveContentEditors(
  references: MemberReference[],
): Promise<ContentEditorLink[]> {
  const editors = await getEntries(references);

  return editors.map((editor) => ({
    id: editor.id,
    label: editor.data.nickname || editor.id,
    href: `/members#${encodeURIComponent(editor.id)}`,
  }));
}

export async function readBlogSummaries(): Promise<BlogSummary[]> {
  const entries = await getCollection('blogs');
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
  const entries = await getCollection('members');
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

export async function readDocsLandingEntry(): Promise<CollectionEntry<'docs'>> {
  const entry = await getEntry('docs', 'index');

  if (!entry) {
    throw new Error('The docs collection must provide contents/docs/index.md.');
  }

  return entry;
}
