import assert from 'node:assert/strict';
import test from 'node:test';

import {
  checkSiteAvailability,
  problemDetail,
  siteAvailabilityTarget,
  SiteSubmissionProblem,
  submissionEndpoint,
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
