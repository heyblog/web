import assert from 'node:assert/strict';
import test from 'node:test';

import { buildSiteDirectorySearchParams } from '../src/api/sites/site-directory.params.ts';
import {
  directoryOptionSelected,
  matchingDirectoryOptions,
  selectedDirectoryOption,
} from '../src/application/site-directory/site-directory.labels.ts';
import { parseSiteDirectorySearchParams } from '../src/application/site-directory/site-directory.shared.ts';

const options = [
  { value: 'JavaScript', label: 'JavaScript', normalCount: 2, abnormalCount: 0 },
  { value: 'JS', label: 'JS', normalCount: 2, abnormalCount: 0 },
  { value: '中文 + / 标签', label: '中文 + / 标签', normalCount: 1, abnormalCount: 0 },
];

test('name search and selection ignore case without merging distinct names', () => {
  assert.deepEqual(matchingDirectoryOptions(options, ' javascript '), [options[0]]);
  assert.equal(directoryOptionSelected(options, options[0], ['JAVASCRIPT']), true);
  assert.equal(directoryOptionSelected(options, options[1], ['JavaScript']), false);
  assert.equal(selectedDirectoryOption(options, ' JAVASCRIPT '), options[0]);
});

test('directory names round trip complete Unicode and punctuation with case-insensitive deduplication', () => {
  const params = new URLSearchParams({ level1: '中文 + / 标签', level2: '字'.repeat(120) });
  for (const name of ['JavaScript', ' JAVASCRIPT ', 'JS', '中文 + / 标签'])
    params.append('tertiary', name);
  params.append('warning', 'Warning');
  params.append('warning', 'warning');
  params.append('level1_label_id', 'retired');
  const query = parseSiteDirectorySearchParams(params);
  assert.deepEqual(query.tertiary, ['JavaScript', 'JS', '中文 + / 标签']);
  assert.deepEqual(query.warning, ['Warning']);
  const serialized = buildSiteDirectorySearchParams(query);
  assert.equal(serialized.has('level1_label_id'), false);
  assert.deepEqual(parseSiteDirectorySearchParams(serialized), query);
  assert.equal(query.level2.length, 120);
});

test('directory rejects oversized names and orphaned secondary classifications', () => {
  const query = parseSiteDirectorySearchParams(
    new URLSearchParams({ level1: '字'.repeat(121), level2: 'child', tertiary: '字'.repeat(121) }),
  );
  assert.equal(query.level1, '');
  assert.equal(query.level2, '');
  assert.deepEqual(query.tertiary, []);
});
