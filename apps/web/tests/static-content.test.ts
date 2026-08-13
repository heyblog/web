import assert from 'node:assert/strict';
import test from 'node:test';

import type {
  BlogSummary,
  MemberSummary,
} from '../src/application/static-content/static-content.models.ts';
import {
  getDocRoutePath,
  shouldSuppressLeadingContentTitle,
  sortBlogSummaries,
  sortMemberSummaries,
} from '../src/application/static-content/static-content.models.ts';

const blog = (input: Partial<BlogSummary> & Pick<BlogSummary, 'id'>): BlogSummary => {
  const { id, ...overrides } = input;

  return {
    id,
    title: id,
    description: '',
    createTime: new Date('2026-01-01T00:00:00Z'),
    category: '公告',
    editors: [],
    tags: [],
    top: false,
    ...overrides,
  };
};

const member = (input: Partial<MemberSummary> & Pick<MemberSummary, 'id'>): MemberSummary => {
  const { id, ...overrides } = input;

  return {
    id,
    displayName: id,
    homepageUrl: `https://example.test/${id}`,
    githubUrl: `https://github.com/${id}`,
    title: '贡献者',
    description: '',
    tags: [],
    status: 'ACTIVE',
    ...overrides,
  };
};

test('sorts blogs by pin, explicit order, date, and id', () => {
  const items = sortBlogSummaries([
    blog({ id: 'later', createTime: new Date('2026-02-01T00:00:00Z') }),
    blog({ id: 'pinned-second', top: true, sort: 2 }),
    blog({ id: 'pinned-first', top: true, sort: 1 }),
    blog({ id: 'earlier', createTime: new Date('2026-01-01T00:00:00Z') }),
  ]);

  assert.deepEqual(
    items.map((item) => item.id),
    ['pinned-first', 'pinned-second', 'later', 'earlier'],
  );
});

test('sorts current members before inactive and alumni entries', () => {
  const items = sortMemberSummaries([
    member({ id: 'alumni', status: 'ALUMNI' }),
    member({ id: 'inactive', status: 'INACTIVE' }),
    member({ id: 'second', sort: 2 }),
    member({ id: 'first', sort: 1 }),
  ]);

  assert.deepEqual(
    items.map((item) => item.id),
    ['first', 'second', 'inactive', 'alumni'],
  );
});

test('maps the docs index to its canonical route', () => {
  assert.equal(getDocRoutePath('index'), '/docs');
  assert.equal(getDocRoutePath('guides/start'), '/docs/guides/start');
});

test('suppresses only a single leading H1 matching the frontmatter title', () => {
  assert.equal(
    shouldSuppressLeadingContentTitle('HeyBlog 文档', [
      { depth: 1, slug: 'heyblog-文档', text: 'HeyBlog 文档' },
      { depth: 2, slug: 'start', text: '开始' },
    ]),
    true,
  );
  assert.equal(
    shouldSuppressLeadingContentTitle('关于我们', [
      { depth: 2, slug: 'about', text: '关于 HeyBlog' },
    ]),
    false,
  );
  assert.throws(
    () =>
      shouldSuppressLeadingContentTitle('HeyBlog 文档', [
        { depth: 1, slug: 'different', text: '其他标题' },
      ]),
    /frontmatter title/u,
  );
});
