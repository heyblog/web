import type { CollectionEntry } from 'astro:content';
import { getCollection, getEntries, getEntry } from 'astro:content';

export type MemberReference =
  | CollectionEntry<'blogs'>['data']['editors'][number]
  | CollectionEntry<'docs'>['data']['editors'][number];

export function readContentEditors(references: MemberReference[]) {
  return getEntries(references);
}

export function readBlogEntries() {
  return getCollection('blogs');
}

export function readPageEntries() {
  return getCollection('pages');
}

export function readDocPageEntries() {
  return getCollection('docs', (entry) => entry.id !== 'index');
}

export function readMemberEntries() {
  return getCollection('members');
}

export async function readDocsLandingEntry(): Promise<CollectionEntry<'docs'>> {
  const entry = await getEntry('docs', 'index');
  if (!entry) {
    throw new Error('The docs collection must provide contents/docs/index.md.');
  }
  return entry;
}
