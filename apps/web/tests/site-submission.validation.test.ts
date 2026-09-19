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
    message: '请填写有效的 HTTP、HTTPS 或站内相对 Feed 地址。',
  });
});

test('rejects every auxiliary URL when it resolves to the homepage', () => {
  const cases = [
    {
      name: 'feed',
      update: (form: ReturnType<typeof emptySubmission>) => {
        form.feeds.push({
          id: 'feed',
          name: '默认订阅',
          url: 'https://example.test/blog#latest',
          format: 'RSS',
          isDefault: true,
        });
      },
      message: 'Feed 地址不能与主页地址相同。',
    },
    {
      name: 'sitemap',
      update: (form: ReturnType<typeof emptySubmission>) => {
        form.sitemap = '/blog';
      },
      message: 'Sitemap 地址不能与主页地址相同。',
    },
    {
      name: 'link page',
      update: (form: ReturnType<typeof emptySubmission>) => {
        form.linkPage = 'http://example.test/blog';
      },
      message: '友链页地址不能与主页地址相同。',
    },
  ] as const;

  for (const testCase of cases) {
    const form = emptySubmission();
    form.url = 'https://example.test/blog';
    testCase.update(form);

    assert.deepEqual(
      validateSubmissionStep('CREATE', form, 1),
      { valid: false, message: testCase.message },
      testCase.name,
    );
  }
});

test('rejects sitemap and link-page URLs that normalize to the same resource', () => {
  const form = emptySubmission();
  form.url = 'https://example.test/blog';
  form.sitemap = '/sitemap.xml';
  form.linkPage = 'https://example.test/sitemap.xml#navigation';

  assert.deepEqual(validateSubmissionStep('CREATE', form, 1), {
    valid: false,
    message: 'Sitemap 和友链页不能使用同一地址。',
  });
});

test('rejects duplicate external resources whose hosts differ only by a trailing dot', () => {
  const form = emptySubmission();
  form.url = 'https://example.test/blog';
  form.sitemap = 'https://resources.example.test./sitemap.xml';
  form.linkPage = 'https://resources.example.test/sitemap.xml';

  assert.deepEqual(validateSubmissionStep('CREATE', form, 1), {
    valid: false,
    message: 'Sitemap 和友链页不能使用同一地址。',
  });
});

test('rejects an auxiliary URL when the homepage host differs only by a trailing dot', () => {
  const form = emptySubmission();
  form.url = 'https://example.test./blog';
  form.sitemap = 'https://example.test/blog';

  assert.deepEqual(validateSubmissionStep('CREATE', form, 1), {
    valid: false,
    message: 'Sitemap 地址不能与主页地址相同。',
  });
});

test('allows omitted or distinct auxiliary URLs', () => {
  const form = emptySubmission();
  form.url = 'https://example.test/blog';

  assert.equal(validateSubmissionStep('CREATE', form, 1).valid, true);

  form.sitemap = 'sitemap.xml';
  form.linkPage = '/friends';
  form.feeds.push({
    id: 'feed',
    name: '默认订阅',
    url: 'feed.xml?format=rss',
    format: 'RSS',
    isDefault: true,
  });
  assert.equal(validateSubmissionStep('CREATE', form, 1).valid, true);
});
