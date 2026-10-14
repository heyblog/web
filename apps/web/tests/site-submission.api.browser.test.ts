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

test('routes every audit action to its dedicated same-origin endpoint', () => {
  assert.equal(submissionEndpoint('CREATE', ''), '/api/web/submissions/create');
  assert.equal(
    submissionEndpoint('UPDATE', existingShortID),
    '/api/web/submissions/A1b2C3d4E/update',
  );
  assert.equal(
    submissionEndpoint('DELETE', existingShortID),
    '/api/web/submissions/A1b2C3d4E/delete',
  );
  assert.equal(
    submissionEndpoint('RESTORE', existingShortID),
    '/api/web/submissions/A1b2C3d4E/restore',
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
    '/api/web/submissions/availability?url=https%3A%2F%2Fexisting.example%2Fblog',
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

test('allows the current site address while preserving the availability request contract', async () => {
  // Given: the address is registered to the site being updated.
  const requested: string[] = [];
  // When: availability is checked with the current site identity.
  const result = await checkSiteAvailability('https://existing.example/blog', {
    currentSiteShortID: existingShortID,
    fetch: async (input) => {
      requested.push(String(input));
      return Response.json({
        available: false,
        existing_site: {
          short_id: existingShortID,
          name: 'Existing Site',
          url: 'https://existing.example/',
          visibility: 'VISIBLE',
        },
      });
    },
  });
  // Then: self is allowed without sending an exclusion parameter upstream.
  assert.equal(result.available, true);
  assert.equal(result.existing_site, undefined);
  assert.deepEqual(requested, [
    '/api/web/submissions/availability?url=https%3A%2F%2Fexisting.example%2Fblog',
  ]);
});

test('blocks another site even when the caller supplies its current site identity', async () => {
  // Given: another site owns the requested address.
  const existing = {
    short_id: 'Z9y8X7w6V',
    name: 'Other Site',
    url: 'https://other.example/',
    visibility: 'VISIBLE',
  };
  // When: availability is checked for the current site.
  const result = await checkSiteAvailability(existing.url, {
    currentSiteShortID: existingShortID,
    fetch: async () => Response.json({ available: false, existing_site: existing }),
  });
  // Then: the conflict keeps the matched site for recovery guidance.
  assert.equal(result.available, false);
  assert.deepEqual(result.existing_site, existing);
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
      error.message === '该主页地址已被其他站点使用，请核对并修改地址。',
  );
});
