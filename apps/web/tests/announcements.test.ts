import assert from 'node:assert/strict';
import test from 'node:test';

import {
  AnnouncementTransportError,
  createAnnouncementTransport,
} from '../src/api/announcements/announcements.browser.ts';
import {
  announcementListQuery,
  forwardAnnouncementManagement,
} from '../src/api/announcements/announcements.proxy.server.ts';
import { isVersion, safeAnnouncementLink } from '../src/api/announcements/announcements.types.ts';
import {
  announcementDraft,
  fromLocalDateTime,
  toLocalDateTime,
} from '../src/application/announcements/announcement-editor.shared.ts';
import { parseAnnouncementInline } from '../src/application/announcements/announcement-inline.shared.ts';
import { createAnnouncementPlayback } from '../src/application/announcements/announcement-playback.browser.ts';
import { validateAnnouncementDraft } from '../src/application/announcements/announcement-validation.shared.ts';

const id = '0194a290-46e0-7000-8000-000000000001';
const fields = {
  kind: 'MAIN',
  title: '公告',
  bodyMarkdown: null,
  priority: 2,
  actionType: 'NONE',
  actionLabel: null,
  actionPath: null,
  actionExternalUrl: null,
  startsAt: null,
  endsAt: null,
} as const;

test('inline emphasis renders while HTML, images and unsafe links remain text', () => {
  const nodes = parseAnnouncementInline(
    '# 标题\n**加粗** *斜体* ~~删除~~ `代码` <script>alert(1)</script>',
  );
  for (const kind of ['strong', 'em', 'del', 'code'])
    assert.ok(nodes.some((node) => node.kind === kind));
  assert.ok(nodes.some((node) => node.kind === 'text' && node.text.includes('<script>')));
  assert.equal(parseAnnouncementInline('![图片](https://example.com/a.png)')[0]?.kind, 'text');
  assert.equal(parseAnnouncementInline('[危险](javascript:alert(1))')[0]?.kind, 'text');
  assert.equal(parseAnnouncementInline('[站内](/announcements)')[0]?.kind, 'link');
});

test('safe targets and integer versions reject unsafe inputs', () => {
  for (const href of [
    `${'java'}script:alert(1)`,
    '//evil.test',
    '/\\evil.test',
    'https://user:pass@example.com',
    'data:text/html,test',
  ])
    assert.equal(safeAnnouncementLink(href, !href.startsWith('/')), false, href);
  assert.equal(safeAnnouncementLink('/announcements?page=2#top', false), true);
  assert.equal(safeAnnouncementLink('https://example.com/path', true), true);
  assert.equal(isVersion('9223372036854775807'), true);
  for (const version of ['0', '-1', '1.0', '9223372036854775808'])
    assert.equal(isVersion(version), false);
});

test('draft maps local timestamps, banner priority and optional action shape', () => {
  const draft = announcementDraft(fields);
  draft.kind = 'BANNER';
  draft.priority = 9;
  draft.actionTarget = 'https://unused.test';
  const result = validateAnnouncementDraft(draft);
  assert.ok(result.kind === 'valid');
  assert.equal(result.payload.priority, 0);
  assert.equal(result.payload.actionExternalUrl, null);
  const iso = '2026-10-03T10:20:30Z';
  assert.equal(fromLocalDateTime(toLocalDateTime(iso)), iso.replace('Z', '.000Z'));
  draft.endsAt = toLocalDateTime(iso);
  assert.equal(validateAnnouncementDraft(draft).kind, 'invalid');
});

test('playback cycles all cards and respects independent pauses and disposal', () => {
  let callback: (() => void) | undefined;
  let scheduled = 0;
  const indices: number[] = [];
  const playback = createAnnouncementPlayback(3, (index) => indices.push(index), {
    set: (next, delay) => {
      assert.equal(delay, 6000);
      callback = next;
      scheduled++;
      return setTimeout(() => {}, 0);
    },
    clear: (timer) => {
      clearTimeout(timer);
      callback = undefined;
    },
  });
  playback.start();
  callback?.();
  callback?.();
  callback?.();
  assert.deepEqual(indices, [1, 2, 0]);
  playback.pause('user', true);
  playback.pause('hover', true);
  playback.pause('hover', false);
  assert.equal(callback, undefined);
  playback.pause('user', false);
  assert.ok(callback);
  playback.move(-1);
  assert.equal(indices.at(-1), 2);
  playback.dispose();
  assert.equal(callback, undefined);
  const old = scheduled;
  playback.start();
  assert.equal(scheduled, old);
});

test('editing content preserves database timestamp precision when displayed time is unchanged', () => {
  const previous = {
    ...fields,
    startsAt: '2026-10-02T08:00:00.123456Z',
    endsAt: '2026-11-02T08:00:00.987654Z',
  };
  const draft = announcementDraft(previous);
  draft.title = '内容修订';
  const result = validateAnnouncementDraft(draft, previous);
  assert.ok(result.kind === 'valid');
  assert.equal(result.payload.startsAt, previous.startsAt);
  assert.equal(result.payload.endsAt, previous.endsAt);
  draft.startsAt = draft.startsAt.slice(0, 16);
  draft.endsAt = draft.endsAt.slice(0, 16);
  const normalized = validateAnnouncementDraft(draft, previous);
  assert.ok(normalized.kind === 'valid');
  assert.equal(normalized.payload.startsAt, previous.startsAt);
  assert.equal(normalized.payload.endsAt, previous.endsAt);
  draft.endsAt = toLocalDateTime('2026-11-03T08:00:00Z');
  const changed = validateAnnouncementDraft(draft, previous);
  assert.ok(changed.kind === 'valid');
  assert.equal(changed.payload.endsAt, '2026-11-03T08:00:00.000Z');
});

test('zero and one card do not create playback timers', () => {
  for (const count of [0, 1]) {
    const playback = createAnnouncementPlayback(count, () => {}, {
      set: () => {
        throw new Error('must not schedule');
      },
      clear: () => {},
    });
    playback.start();
    playback.pause('hidden', false);
    playback.move(1);
    playback.dispose();
  }
});

test('boundary rejects invalid queries, paths, methods, origins and oversized bodies', async () => {
  assert.equal(
    announcementListQuery(new URL('https://hey.test/?page=2&kind=MAIN')),
    '?page=2&kind=MAIN',
  );
  for (const query of ['page=0', 'page=1&page=2', 'pageSize=101', 'status=ACTIVE', 'unknown=x'])
    assert.equal(announcementListQuery(new URL(`https://hey.test/?${query}`)), null);
  const upstream = async () => {
    throw new Error('must not forward');
  };
  const base = 'https://hey.test/management/announcements/data';
  assert.equal(
    (await forwardAnnouncementManagement(new Request(base), '../users', upstream)).status,
    404,
  );
  assert.equal(
    (await forwardAnnouncementManagement(new Request(base, { method: 'PUT' }), '', upstream))
      .status,
    405,
  );
  assert.equal(
    (
      await forwardAnnouncementManagement(
        new Request(base, { method: 'POST', body: JSON.stringify(fields) }),
        '',
        upstream,
      )
    ).status,
    403,
  );
  for (const body of [
    'invalid',
    JSON.stringify({ ...fields, extra: true }),
    JSON.stringify({ ...fields, bodyMarkdown: 'x'.repeat(64001) }),
  ]) {
    const request = new Request(base, {
      method: 'POST',
      body,
      headers: {
        'Content-Type': 'application/json',
        'Sec-Fetch-Site': 'same-origin',
        Origin: 'https://hey.test',
      },
    });
    assert.equal((await forwardAnnouncementManagement(request, '', upstream)).status, 422);
  }
});

test('valid deletion forwards once with version and strips unsafe headers', async () => {
  let calls = 0;
  const request = new Request(`https://hey.test/management/announcements/data/${id}`, {
    method: 'DELETE',
    body: JSON.stringify({ rowVersion: '2' }),
    headers: {
      'Content-Type': 'application/json',
      'Sec-Fetch-Site': 'same-origin',
      Origin: 'https://hey.test',
    },
  });
  const response = await forwardAnnouncementManagement(
    request,
    id,
    async (incoming, path, init) => {
      calls++;
      assert.equal(incoming, request);
      assert.equal(path, `/management/announcements/${id}`);
      assert.equal(init?.method, 'DELETE');
      assert.deepEqual(init?.body, { rowVersion: '2' });
      return new Response(null, { status: 204, headers: { Location: 'https://internal.test/' } });
    },
  );
  assert.equal(calls, 1);
  assert.equal(response.status, 204);
  assert.equal(response.headers.get('Cache-Control'), 'private, no-store');
  assert.equal(response.headers.get('Location'), null);
});

test('transport rejects malformed success and exposes typed conflict without retries', async () => {
  await assert.rejects(
    createAnnouncementTransport(async () => Response.json({ announcement: {} })).create(fields),
    (error: unknown) =>
      error instanceof AnnouncementTransportError && error.code === 'invalid_response',
  );
  let calls = 0;
  await assert.rejects(
    createAnnouncementTransport(async () => {
      calls++;
      return Response.json({ code: 'row_version_conflict' }, { status: 409 });
    }).archive(id, '1'),
    (error: unknown) => error instanceof AnnouncementTransportError && error.status === 409,
  );
  assert.equal(calls, 1);
});
