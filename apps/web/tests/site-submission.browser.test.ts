import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

import {
  addFeed,
  buildSubmissionPayload,
  emptySubmission,
  removeFeed,
  setDefaultFeed,
} from '../src/application/site-submission/site-submission.browser.ts';
import { matchesSubmissionOption } from '../src/application/site-submission/site-submission.search.ts';
import { isSiteShortID } from '../src/application/site-submission/site-submission.validation.ts';

test('keeps the submission stepper compatible with the production CSP', async () => {
  const source = await readFile(
    new URL('../src/components/site-submission/SubmissionStepper.svelte', import.meta.url),
    'utf8',
  );

  assert.doesNotMatch(source, /\bstyle(?::|=)/u);
});

test('accepts only fixed-width case-sensitive Base62 site short IDs', () => {
  assert.equal(isSiteShortID('A1b2C3d4E'), true);
  assert.equal(isSiteShortID('550e8400-e29b-41d4-a716-446655440000'), false);
  assert.equal(isSiteShortID('custom-id'), false);
  assert.equal(isSiteShortID('A1b2C3d4'), false);
});

test('builds a structured custom-program submission payload', () => {
  const form = emptySubmission();
  form.name = ' Example Blog ';
  form.url = ' https://example.test/blog ';
  form.summary = ' Summary ';
  form.feeds = [
    { id: 'feed-1', name: ' Main ', url: ' /feed.xml ', format: 'RSS', isDefault: true },
  ];
  form.sitemap = ' https://example.test/sitemap.xml ';
  form.tags = [
    { id: 'tag-primary', name: '中文博客', role: 'PRIMARY' },
    { id: 'tag-secondary', name: '独立开发', role: 'SECONDARY' },
  ];
  form.program = {
    kind: 'custom',
    name: ' Example Engine ',
    isOpenSource: true,
    homepageURL: '',
    repositoryURL: ' https://code.example/engine ',
    dependencies: [
      { id: 'component-program', name: 'Astro', role: 'FRAMEWORK' },
      { id: '', name: ' TypeScript ', role: 'LANGUAGE' },
    ],
  };
  form.reason = ' Please add this site. ';
  form.contactEmail = ' owner@example.test ';
  form.notifyByEmail = true;

  const payload = buildSubmissionPayload(form, 'UPDATE');

  assert.deepEqual(payload.site.tags, [
    { id: 'tag-primary', suggested_name: '', slug: '', description: '', role: 'PRIMARY' },
    { id: 'tag-secondary', suggested_name: '', slug: '', description: '', role: 'SECONDARY' },
  ]);
  assert.deepEqual(payload.site.components, [
    {
      id: '',
      suggested_name: 'Example Engine',
      role: 'SITE_PROGRAM',
      homepage_url: 'https://code.example/engine',
      repository_url: 'https://code.example/engine',
      is_open_source: true,
    },
  ]);
  assert.deepEqual(payload.site.program_dependencies, [
    {
      id: 'component-program',
      suggested_name: '',
      role: 'FRAMEWORK',
      homepage_url: '',
      repository_url: '',
      is_open_source: null,
    },
    {
      id: '',
      suggested_name: 'TypeScript',
      role: 'LANGUAGE',
      homepage_url: '',
      repository_url: '',
      is_open_source: null,
    },
  ]);
  assert.deepEqual(payload.site.feeds, [
    { name: 'Main', url: '/feed.xml', format: 'RSS', is_default: true },
  ]);
});

test('omits the reason field for a create submission', () => {
  const form = emptySubmission();
  form.name = 'Example Blog';
  form.url = 'https://example.test';
  form.tags = [{ id: 'tag-primary', name: '中文博客', role: 'PRIMARY' }];
  form.program = { kind: 'other', id: 'private-program', name: '其他' };

  const payload = buildSubmissionPayload(form, 'CREATE');

  assert.equal('reason' in payload, false);
});

test('maintains exactly one default feed while adding and removing feeds', () => {
  const form = emptySubmission();
  addFeed(form);
  addFeed(form);
  const secondID = form.feeds[1]?.id;
  assert.ok(secondID);
  setDefaultFeed(form, secondID);
  removeFeed(form, secondID);
  assert.equal(form.feeds.length, 1);
  assert.equal(form.feeds[0]?.isDefault, true);
});

test('does not infer auxiliary resources from the homepage', () => {
  const form = emptySubmission();
  form.url = 'https://new.example';

  const payload = buildSubmissionPayload(form, 'CREATE');

  assert.deepEqual(payload.site.feeds, []);
  assert.deepEqual(payload.site.resources, []);
});

test('matches CJK and punctuation-insensitive submission options', () => {
  assert.equal(matchesSubmissionOption('中 文', '中文博客'), true);
  assert.equal(matchesSubmissionOption('astro', 'Astro.js'), true);
  assert.equal(matchesSubmissionOption('atjs', 'Astro.js'), true);
  assert.equal(matchesSubmissionOption('vue', 'Astro.js'), false);
});
