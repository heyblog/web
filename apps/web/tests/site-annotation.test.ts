import assert from 'node:assert/strict';
import test from 'node:test';

import type { SessionUser } from '../src/api/auth/auth.types.ts';
import { forwardAnnotation } from '../src/api/site-annotation/annotation.server.ts';
import { parseAnnotationDetail } from '../src/api/site-annotation/annotation-responses.ts';
import { managementNavigation } from '../src/application/management/navigation.ts';
import { canAnnotateSites } from '../src/application/site-annotation/annotation-access.ts';
import {
  annotationDraft,
  annotationPatch,
} from '../src/application/site-annotation/annotation-draft.ts';

test('an administrator can have annotation access without management workspace access', () => {
  const user: Pick<SessionUser, 'role' | 'permissions'> = {
    role: 'ADMIN',
    permissions: ['site_annotation.manage'],
  };
  assert.equal(canAnnotateSites(user), true);
  assert.deepEqual(managementNavigation(user), []);
  assert.equal(canAnnotateSites({ ...user, permissions: ['site.manage'] }), false);
});

const detail = parseAnnotationDetail({
  short_id: 'site123',
  name: 'Example',
  url: 'https://example.test',
  summary: 'About',
  blog_status: 'UNREVIEWED',
  access_scope: 'ALL',
  revision: 7,
  updated_at: '2026-11-18T12:00:00Z',
  friend_version: 'a'.repeat(64),
  tags: [
    { id: 'primary', name: '一级分类', role: 'PRIMARY', level: 1 },
    { id: 'topic', name: '主题标签', role: 'TERTIARY', level: 3 },
  ],
  feeds: [
    {
      id: 'feed123',
      name: 'Example RSS',
      url: 'https://example.test/rss',
      format: 'RSS',
      is_default: true,
      is_enabled: true,
    },
  ],
  components: [],
  resources: [],
  outgoing: [
    { target_host: 'other.test', target_url: 'https://other.test', link_status: 'ACTIVE' },
  ],
  incoming: [
    {
      source_short_id: 'source123',
      source_name: 'Other',
      source_url: 'https://source.test',
      link_status: 'ACTIVE',
    },
  ],
});

test('partial annotation requests preserve saved versions and send explicit removals', () => {
  const draft = annotationDraft(detail);
  assert.equal(annotationPatch(draft, detail, 'reason alone'), null);

  const [tag] = draft.tags;
  const [feed] = draft.feeds;
  assert.ok(tag && feed);
  tag.role = 'WARNING';
  feed.name = 'RSS';
  draft.outgoing = [];
  assert.deepEqual(annotationPatch(draft, detail, '  Reviewed  '), {
    expected_revision: detail.revision,
    friend_version: detail.friend_version,
    reason: 'Reviewed',
    tags: [{ id: 'topic', role: 'WARNING' }],
    feeds: [
      {
        id: 'feed123',
        name: 'RSS',
        url: 'https://example.test/rss',
        format: 'RSS',
        is_default: true,
        is_enabled: true,
      },
    ],
    outgoing: [],
  });
});

const browserHeaders = {
  Accept: 'application/json',
  'Sec-Fetch-Site': 'same-origin',
  'Sec-Fetch-Mode': 'same-origin',
  'Sec-Fetch-Dest': 'empty',
};

test('annotation HTTP boundary denies cross-origin reads, navigation, unsupported paths and unbounded writes', async () => {
  const url = 'https://web.test/api/web/management/site-annotations';
  assert.equal((await forwardAnnotation(new Request(url), '')).status, 403);
  assert.equal(
    (
      await forwardAnnotation(
        new Request(url, {
          headers: {
            ...browserHeaders,
            Origin: 'https://elsewhere.test',
          },
        }),
        '',
      )
    ).status,
    403,
  );
  assert.equal(
    (
      await forwardAnnotation(
        new Request(url, {
          headers: {
            ...browserHeaders,
            'Sec-Fetch-Dest': 'document',
          },
        }),
        '',
      )
    ).status,
    403,
  );
  assert.equal(
    (
      await forwardAnnotation(
        new Request(url, { headers: browserHeaders }),
        'site123/history/extra',
      )
    ).status,
    404,
  );
  const patch = (body: string) =>
    new Request(`${url}/site123`, {
      method: 'PATCH',
      headers: {
        ...browserHeaders,
        Origin: 'https://web.test',
        'Content-Type': 'application/json',
      },
      body,
    });
  assert.equal((await forwardAnnotation(patch('{}'), 'options')).status, 404);
  assert.equal((await forwardAnnotation(patch('[]'), 'site123')).status, 400);
  assert.equal(
    (await forwardAnnotation(patch(JSON.stringify({ summary: '接'.repeat(100_000) })), 'site123'))
      .status,
    413,
  );
});

test('annotation HTTP boundary forwards only permitted query and mutation fields', async (context) => {
  context.mock.method(globalThis, 'fetch', async (input: RequestInfo | URL, init?: RequestInit) => {
    requests.push(new Request(input, init));
    return Response.json(
      {},
      { headers: { Location: 'https://elsewhere.test', 'Set-Cookie': 'session=new; HttpOnly' } },
    );
  });
  const requests: Request[] = [];
  const url = 'https://web.test/api/web/management/site-annotations';
  const originalOrigin = process.env.WEB_API_BASE_URL;
  const originalToken = process.env.API_WEB_TOKEN;
  context.after(() => {
    if (originalOrigin === undefined) delete process.env.WEB_API_BASE_URL;
    else process.env.WEB_API_BASE_URL = originalOrigin;
    if (originalToken === undefined) delete process.env.API_WEB_TOKEN;
    else process.env.API_WEB_TOKEN = originalToken;
  });
  process.env.WEB_API_BASE_URL = 'http://api.internal';
  process.env.API_WEB_TOKEN = 'test-web-service-token-0123456789abcdef';
  await forwardAnnotation(
    new Request(`${url}?status=&page=2&q=%E5%8D%9A%E5%AE%A2&extra=x`, {
      headers: browserHeaders,
    }),
    '',
  );
  assert.equal(
    requests[0]?.url,
    'http://api.internal/api/web/management/site-annotations?status=UNREVIEWED&q=%E5%8D%9A%E5%AE%A2&page=2',
  );

  const response = await forwardAnnotation(
    new Request(`${url}/site123?q=wrong`, {
      method: 'PATCH',
      headers: {
        ...browserHeaders,
        Origin: 'https://web.test',
        'Content-Type': 'application/json',
        Cookie: 'session=old',
      },
      body: JSON.stringify({
        expected_revision: 7,
        friend_version: detail.friend_version,
        feeds: [],
        incoming: [],
        visibility: 'REMOVED',
      }),
    }),
    'site123',
  );
  const forwarded = requests[1];
  assert.equal(forwarded?.url, 'http://api.internal/api/web/management/site-annotations/site123');
  assert.equal(forwarded?.method, 'PATCH');
  assert.equal(forwarded?.headers.get('Cookie'), 'session=old');
  assert.deepEqual(await forwarded?.json(), {
    expected_revision: 7,
    friend_version: detail.friend_version,
    feeds: [],
  });
  assert.equal(response.headers.get('Cache-Control'), 'private, no-store');
  assert.equal(response.headers.get('Location'), null);
  assert.deepEqual(response.headers.getSetCookie(), ['session=new; HttpOnly']);
});
