import assert from 'node:assert/strict';
import test from 'node:test';

import { buildSiteDirectorySearchParams } from '../src/api/sites/site-directory.params.ts';
import {
  directoryOptionSelected,
  matchingDirectoryOptions,
} from '../src/application/site-directory/site-directory.labels.ts';
import { parseSiteDirectorySearchParams } from '../src/application/site-directory/site-directory.shared.ts';

const zh = '019f033c-2111-7000-9000-000000000001';
const en = '019f033c-2111-7000-9000-000000000002';
const options = [
  { value: 'algorithm', label_id: zh, label: '算法', normalCount: 2, abnormalCount: 0 },
  { value: 'algorithm', label_id: en, label: 'algorithm', normalCount: 2, abnormalCount: 0 },
  { value: 'life', label: '生活', normalCount: 1, abnormalCount: 0 },
];

test('either name expands the group while selection uses a single presentation label', () => {
  assert.deepEqual(
    matchingDirectoryOptions(options, '算法').map((option) => option.label_id),
    [zh, en],
  );
  assert.equal(directoryOptionSelected(options, options[0], ['algorithm'], [en]), false);
  assert.equal(directoryOptionSelected(options, options[1], ['algorithm'], [en]), true);
  assert.equal(directoryOptionSelected(options, options[0], ['algorithm']), true);
  assert.equal(directoryOptionSelected(options, options[1], ['algorithm']), false);
});

test('presentation IDs round trip separately from canonical slug filters', () => {
  const params = new URLSearchParams(
    `level1=algorithm&level1_label_id=${zh}&level2=algorithm&level2_label_id=${en}&tertiary=algorithm&tertiary=algorithm&tertiary_label_id=${en}&tertiary_label_id=invalid`,
  );
  const query = parseSiteDirectorySearchParams(params);
  assert.deepEqual(query.tertiary, ['algorithm']);
  assert.deepEqual(query.tertiary_label_ids, [en]);
  const restored = parseSiteDirectorySearchParams(buildSiteDirectorySearchParams(query));
  assert.equal(restored.level1_label_id, zh);
  assert.equal(restored.level2_label_id, en);
  assert.deepEqual(restored.tertiary_label_ids, [en]);
});
