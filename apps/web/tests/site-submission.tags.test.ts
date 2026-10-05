import assert from 'node:assert/strict';
import test from 'node:test';

import {
  buildSubmissionPayload,
  emptySubmission,
} from '../src/application/site-submission/site-submission.browser.ts';
import {
  removeTag,
  selectClassificationTag,
  selectTertiaryTag,
  tertiaryTagOptions,
} from '../src/application/site-submission/site-submission.tags.browser.ts';

test('deduplicates fixed dictionary entries while excluding selected tags and warnings', () => {
  const form = emptySubmission();
  selectClassificationTag(form, { id: 'computer', name: '计算机', level: 1 });
  selectTertiaryTag(form, { id: 'selected', name: '已选', level: 3 });
  const result = tertiaryTagOptions(
    [
      { id: 'computer', name: '计算机', level: 1 },
      { id: 'computer', name: '计算机', level: 3 },
      { id: 'life', name: '生活', level: 1 },
      { id: 'life', name: '生活', level: 3 },
      { id: 'life-duplicate', name: ' 生活 ', level: 3 },
      { id: 'warning', name: '提醒', role: 'WARNING' },
      { id: 'selected', name: '已选', level: 3 },
    ],
    form.tags,
  );

  assert.deepEqual(
    result.map((tag) => tag.id),
    ['life'],
  );
});

test('replaces a linked classification without removing unrelated tertiary tags', () => {
  const form = emptySubmission();
  selectClassificationTag(form, { id: 'computer', name: '计算机', level: 1 });
  selectClassificationTag(form, {
    id: 'development',
    name: '开发',
    level: 2,
    parent_id: 'computer',
  });
  selectTertiaryTag(form, { id: 'writing', name: '写作', level: 1 });
  selectClassificationTag(form, { id: 'life', name: '生活', level: 1 });

  assert.deepEqual(
    form.tags.map((tag) => [tag.id, tag.role, tag.level]),
    [
      ['writing', 'TERTIARY', 3],
      ['life', 'PRIMARY', 1],
    ],
  );
});

test('prevents classification tags from being selected twice', () => {
  const form = emptySubmission();
  selectClassificationTag(form, { id: 'computer', name: '计算机', level: 1 });
  selectClassificationTag(form, {
    id: 'development',
    name: '开发',
    level: 2,
    parent_id: 'computer',
  });
  selectTertiaryTag(form, { id: 'computer', name: '计算机', level: 1 });
  selectTertiaryTag(form, { id: 'development', name: '开发', level: 2 });

  assert.equal(form.tags.length, 2);
});

test('serializes fixed and custom tertiary tags at level three', () => {
  const form = emptySubmission();
  selectClassificationTag(form, { id: 'computer', name: '计算机', level: 1 });
  selectClassificationTag(form, {
    id: 'development',
    name: '开发',
    level: 2,
    parent_id: 'computer',
  });
  selectTertiaryTag(form, { id: 'writing', name: '写作', level: 1 });
  selectTertiaryTag(form, { id: 'custom-local', name: '编译器', level: 3, is_custom: true });

  const customDraft = form.tags.find((tag) => tag.suggestedName === '编译器');
  assert.ok(customDraft);
  customDraft.slug = 'compiler';
  customDraft.description = '编译器相关内容';
  const payload = buildSubmissionPayload(form, 'CREATE');

  assert.deepEqual(
    payload.site.tags.map((tag) => [tag.id, tag.suggested_name, tag.role, tag.level]),
    [
      ['computer', '', 'PRIMARY', 1],
      ['development', '', 'SECONDARY', 2],
      ['writing', '', 'TERTIARY', 3],
      ['', '编译器', 'TERTIARY', 3],
    ],
  );
});

test('limits tertiary selections to twenty and removes them independently', () => {
  const form = emptySubmission();
  for (let index = 0; index < 21; index += 1) {
    selectTertiaryTag(form, { id: `tag-${index}`, name: `标签 ${index}`, level: 3 });
  }
  removeTag(form, 'tag-4');

  assert.equal(form.tags.filter((tag) => tag.role === 'TERTIARY').length, 19);
  assert.equal(
    form.tags.some((tag) => tag.id === 'tag-20'),
    false,
  );
});

test('classification edits use assignment roles when historical tertiary metadata has dictionary levels', () => {
  const form = emptySubmission();
  selectClassificationTag(form, { id: 'computer', name: '计算机', level: 1 });
  selectClassificationTag(form, {
    id: 'development',
    name: '开发',
    level: 2,
    parent_id: 'computer',
  });
  form.tags.push({ id: 'writing', name: '写作', role: 'TERTIARY', level: 1 });
  form.tags.push({ id: 'reading', name: '阅读', role: 'TERTIARY', level: 2 });
  removeTag(form, 'writing');
  assert.equal(
    form.tags.some((tag) => tag.id === 'development'),
    true,
  );
  selectClassificationTag(form, { id: 'life', name: '生活', level: 1 });
  assert.deepEqual(
    form.tags.map((tag) => tag.id),
    ['reading', 'life'],
  );
});

test('same dictionary ID can fill both classification roles and secondary removal preserves primary', () => {
  const form = emptySubmission();
  selectClassificationTag(form, { id: 'other', name: '其他', level: 1 });
  selectClassificationTag(form, { id: 'other', name: '其他', level: 2, parent_id: 'other' });
  assert.deepEqual(
    form.tags.map((tag) => tag.role),
    ['PRIMARY', 'SECONDARY'],
  );
  assert.deepEqual(
    buildSubmissionPayload(form, 'CREATE').site.tags.map((tag) => [tag.id, tag.role]),
    [
      ['other', 'PRIMARY'],
      ['other', 'SECONDARY'],
    ],
  );
  selectTertiaryTag(form, { id: 'other', name: '其他', level: 3 });
  assert.equal(form.tags.length, 2);
  removeTag(form, 'other', 'SECONDARY');
  assert.deepEqual(
    form.tags.map((tag) => tag.role),
    ['PRIMARY'],
  );
});

test('tertiary exclusion only concerns the selected object classification', () => {
  const form = emptySubmission();
  selectClassificationTag(form, { id: 'computer', name: '计算机', level: 1 });
  selectClassificationTag(form, {
    id: 'development',
    name: '开发',
    level: 2,
    parent_id: 'computer',
  });
  selectTertiaryTag(form, { id: 'life', name: '生活', level: 1 });
  assert.equal(form.tags.find((tag) => tag.id === 'life')?.role, 'TERTIARY');
  selectClassificationTag(form, { id: 'life', name: '生活', level: 1 });
  assert.deepEqual(
    form.tags.map((tag) => [tag.id, tag.role]),
    [['life', 'PRIMARY']],
  );
});
