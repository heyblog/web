import assert from 'node:assert/strict';
import test from 'node:test';

import { emptySubmission } from '../src/application/site-submission/site-submission.browser.ts';
import { applySnapshot } from '../src/application/site-submission/site-submission.snapshot.browser.ts';
import type { SubmissionOptions } from '../src/application/site-submission/site-submission.types.ts';

const options: SubmissionOptions = {
  tags: [{ id: 'tag-primary', name: '中文博客' }],
  components: [
    {
      id: 'component-program',
      name: 'Astro',
      homepage_url: 'https://astro.build',
      repository_url: 'https://github.com/withastro/astro',
      is_open_source: true,
    },
  ],
  program_dependencies: [],
  private_program_id: 'private-program',
};

test('applies a complete aggregate snapshot to editable state', () => {
  const form = emptySubmission();
  applySnapshot(
    form,
    {
      short_id: 'A1b2C3d4E',
      revision: 4,
      name: 'Example',
      scheme: 'https',
      normalized_host: 'example.test',
      base_path: '/blog',
      summary: 'Summary',
      access_scope: 'ALL',
      visibility: 'VISIBLE',
      feeds: [{ name: 'Main', url: '/feed.xml', format: 'RSS', is_default: true }],
      resources: [{ kind: 'LINK_PAGE', url: '/friends' }],
      tags: [
        {
          id: 'tag-primary',
          name: '中文博客',
          suggested_name: '',
          slug: 'chinese-blog',
          description: '',
          role: 'PRIMARY',
        },
      ],
      components: [
        {
          id: 'component-program',
          name: 'Astro',
          suggested_name: '',
          role: 'SITE_PROGRAM',
          homepage_url: 'https://astro.build',
          repository_url: 'https://github.com/withastro/astro',
          is_open_source: true,
        },
      ],
      program_dependencies: [],
    },
    options,
  );
  assert.equal(form.siteShortId, 'A1b2C3d4E');
  assert.equal(form.url, 'https://example.test/blog');
  assert.equal(form.feeds[0]?.url, '/feed.xml');
  assert.equal(form.linkPage, '/friends');
  assert.equal(form.tags[0]?.name, '中文博客');
  assert.deepEqual(form.program, {
    kind: 'existing',
    id: 'component-program',
    name: 'Astro',
    dependencies: [],
  });
});

test('restores a custom program and its dependencies from an audit snapshot', () => {
  const form = emptySubmission();
  applySnapshot(
    form,
    {
      name: 'Example',
      scheme: 'https',
      normalized_host: 'example.test',
      base_path: '/',
      summary: '',
      access_scope: 'ALL',
      visibility: 'VISIBLE',
      feeds: [],
      resources: [],
      tags: [
        {
          id: 'tag-primary',
          name: '中文博客',
          suggested_name: '',
          slug: 'chinese-blog',
          description: '',
          role: 'PRIMARY',
        },
      ],
      components: [
        {
          id: '',
          name: '',
          suggested_name: 'Custom Engine',
          role: 'SITE_PROGRAM',
          homepage_url: 'https://engine.example',
          repository_url: 'https://code.example/engine',
          is_open_source: true,
        },
      ],
      program_dependencies: [
        {
          id: '',
          name: '',
          suggested_name: 'Custom Runtime',
          role: 'LANGUAGE',
          homepage_url: 'https://runtime.example',
          repository_url: 'https://code.example/runtime',
          is_open_source: true,
        },
      ],
    },
    options,
  );

  assert.deepEqual(form.program, {
    kind: 'custom',
    name: 'Custom Engine',
    isOpenSource: true,
    homepageURL: 'https://engine.example',
    repositoryURL: 'https://code.example/engine',
    dependencies: [
      {
        id: '',
        name: 'Custom Runtime',
        role: 'LANGUAGE',
        isOpenSource: true,
        homepageURL: 'https://runtime.example',
        repositoryURL: 'https://code.example/runtime',
      },
    ],
  });
});
