import assert from 'node:assert/strict';
import test from 'node:test';

import { submissionEndpoint } from '../src/api/site-submission/site-submission.browser.ts';
import {
  checkSiteAvailability,
  problemDetail,
  siteAvailabilityTarget,
  SiteSubmissionProblem,
  submitForm,
} from '../src/application/site-submission/site-submission.api.browser.ts';
import { emptySubmission } from '../src/application/site-submission/site-submission.browser.ts';

const existingShortID = 'A1b2C3d4E';

test('maps stable API problem codes without exposing upstream detail text', async () => {
  const response = Response.json(
    { code: 'submission_no_changes', detail: 'database lookup failed at internal host' },
    { status: 422 },
  );
  const message = await problemDetail(response);
  assert.equal(message, '未检测到可提交的修改。');
  assert.doesNotMatch(message, /database|internal/);
});

test('maps URL purpose conflicts to actionable form guidance', async () => {
  const response = Response.json({ code: 'site_url_purpose_conflict' }, { status: 422 });

  assert.equal(
    await problemDetail(response),
    '主页、Feed、Sitemap 和友链页不能使用同一地址，请修改地址或清空可选项。',
  );
});

test('gives recovery guidance for review slug collisions and unavailable names', async () => {
  for (const [code, expected] of [
    [
      'slug_needs_confirmation',
      '新标签的 slug 与已有标签冲突。相同含义请在标签管理中添加同义名称或合并标签；不同含义请填写独立 slug 后重新审核。',
    ],
    ['invalid_tag_label', '所选标签名称已不可用，请刷新后重新选择。'],
  ]) {
    assert.equal(
      await problemDetail(
        Response.json({ code, detail: 'internal database failure' }, { status: 409 }),
      ),
      expected,
    );
  }
});

test('shows validated review collision candidates without raw diagnostics', async () => {
  const message = await problemDetail(
    Response.json(
      {
        code: 'slug_needs_confirmation',
        detail: 'internal host failure',
        invalid_params: [
          { name: 'tags.算法 algorithm.slug', reason: 'algorithm' },
          { name: 'conflicts.019f033c-2111-7000-9000-000000000001', reason: '算法 (algorithm)' },
          { name: 'database', reason: 'internal host failure' },
          { name: 'tags.invalid.slug', reason: 'invalid candidate!' },
        ],
      },
      { status: 409 },
    ),
  );
  assert.match(message, /候选 slug：algorithm/);
  assert.match(message, /冲突标签：算法 \(algorithm\)/);
  assert.doesNotMatch(message, /internal|invalid candidate/);
});

test('routes every audit action to its dedicated same-origin endpoint', () => {
  assert.equal(submissionEndpoint('CREATE', ''), '/api/site-submissions/create');
  assert.equal(
    submissionEndpoint('UPDATE', existingShortID),
    '/api/site-submissions/A1b2C3d4E/update',
  );
  assert.equal(
    submissionEndpoint('DELETE', existingShortID),
    '/api/site-submissions/A1b2C3d4E/delete',
  );
  assert.equal(
    submissionEndpoint('RESTORE', existingShortID),
    '/api/site-submissions/A1b2C3d4E/restore',
  );
});

test('checks exact site availability through the same-origin boundary', async () => {
  let requestedURL = '';
  const availability = await checkSiteAvailability('https://existing.example/blog', {
    fetch: async (input) => {
      requestedURL = String(input);
      return Response.json({
        available: false,
        existing_site: {
          short_id: existingShortID,
          name: 'Existing Site',
          url: 'https://existing.example.com',
          visibility: 'VISIBLE',
        },
      });
    },
  });

  assert.equal(
    requestedURL,
    '/api/site-submissions/availability?url=https%3A%2F%2Fexisting.example%2Fblog',
  );
  assert.equal(availability.available, false);
  assert.equal(availability.existing_site?.short_id, existingShortID);
});

test('routes duplicate sites to update or restore by lifecycle state', () => {
  const existing = {
    short_id: existingShortID,
    name: 'Existing Site',
    url: 'https://existing.example',
    visibility: 'VISIBLE',
  } as const;

  assert.equal(siteAvailabilityTarget(existing), '/site/submissions/update?short_id=A1b2C3d4E');
  assert.equal(
    siteAvailabilityTarget({ ...existing, visibility: 'REMOVED' }),
    '/site/submissions/restore?short_id=A1b2C3d4E',
  );
});

test('preserves the stable problem code when submission races with registration', async () => {
  const form = emptySubmission();
  await assert.rejects(
    submitForm('CREATE', form, {
      fetch: async () => Response.json({ code: 'site_address_conflict' }, { status: 409 }),
    }),
    (error: unknown) =>
      error instanceof SiteSubmissionProblem &&
      error.code === 'site_address_conflict' &&
      error.message === '该站点已在目录中，请改为提交更新申请。',
  );
});
