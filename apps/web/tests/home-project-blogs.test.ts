import assert from 'node:assert/strict';
import test from 'node:test';

import { selectHomeProjectBlogs } from '../src/application/home/home-project-blogs.shared.ts';
import type { BlogSummary } from '../src/application/static-content/static-content.models.ts';

const blog = (id: string): BlogSummary => ({
  id,
  title: id,
  description: '',
  createTime: new Date('2026-01-01T00:00:00Z'),
  category: '公告',
  editors: [],
  tags: [],
  top: false,
});

test('selects at most the first three sorted project blogs', () => {
  const selected = selectHomeProjectBlogs([
    blog('first'),
    blog('second'),
    blog('third'),
    blog('fourth'),
  ]);

  assert.deepEqual(
    selected.map((item) => item.id),
    ['first', 'second', 'third'],
  );
});

test('keeps every project blog when fewer than three are available', () => {
  const selected = selectHomeProjectBlogs([blog('first'), blog('second')]);

  assert.deepEqual(
    selected.map((item) => item.id),
    ['first', 'second'],
  );
});
