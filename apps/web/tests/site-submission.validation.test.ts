import assert from 'node:assert/strict';
import test from 'node:test';

import { emptySubmission } from '../src/application/site-submission/site-submission.browser.ts';
import { selectClassificationTag } from '../src/application/site-submission/site-submission.tags.browser.ts';
import { validateSubmissionStep } from '../src/application/site-submission/site-submission.validation.ts';

test('requires one linked first- and second-level classification', () => {
  const form = emptySubmission();
  form.program = { kind: 'other', id: 'other', name: '其他' };
  selectClassificationTag(form, { id: 'computer', name: '计算机', level: 1 });

  assert.equal(validateSubmissionStep('CREATE', form, 2).valid, false);

  selectClassificationTag(form, {
    id: 'development',
    name: '开发',
    level: 2,
    parent_id: 'different-parent',
  });
  assert.equal(validateSubmissionStep('CREATE', form, 2).valid, false);

  selectClassificationTag(form, {
    id: 'development',
    name: '开发',
    level: 2,
    parent_id: 'computer',
  });
  assert.equal(validateSubmissionStep('CREATE', form, 2).valid, true);
});

test('requires contact name and email to be supplied together', () => {
  const form = emptySubmission();

  assert.equal(validateSubmissionStep('CREATE', form, 3).valid, true);

  form.contactName = '站长';
  assert.equal(validateSubmissionStep('CREATE', form, 3).valid, false);

  form.contactName = '';
  form.contactEmail = 'owner@example.test';
  assert.equal(validateSubmissionStep('CREATE', form, 3).valid, false);

  form.contactName = '站长';
  assert.equal(validateSubmissionStep('CREATE', form, 3).valid, true);
});

test('requires paired contact details when email notifications are enabled', () => {
  const form = emptySubmission();
  form.notifyByEmail = true;

  assert.equal(validateSubmissionStep('CREATE', form, 3).valid, false);
});

test('reports malformed feed addresses without throwing', () => {
  const form = emptySubmission();
  form.url = 'https://example.test';
  form.feeds.push({
    id: 'feed',
    name: '默认订阅',
    url: 'http://[invalid',
    format: 'RSS',
    isDefault: true,
  });

  assert.deepEqual(validateSubmissionStep('CREATE', form, 1), {
    valid: false,
    message: '请补全每个 Feed 的名称和有效地址。',
  });
});
