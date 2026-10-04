import assert from 'node:assert/strict';
import test from 'node:test';

import {
  announcementDraft,
  toLocalDateTime,
} from '../src/application/announcements/announcement-editor.shared.ts';
import {
  validateAnnouncementDraft,
  validateAnnouncementPublish,
} from '../src/application/announcements/announcement-validation.shared.ts';

test('empty and malformed drafts identify fields instead of one generic error', () => {
  const draft = announcementDraft(null);
  draft.priority = undefined;
  draft.actionType = 'INTERNAL';
  draft.actionTarget = 'https://example.com';
  const result = validateAnnouncementDraft(draft);
  assert.ok(result.kind === 'invalid');
  assert.deepEqual(Object.keys(result.fieldErrors).sort(), [
    'actionLabel',
    'actionTarget',
    'priority',
    'title',
  ]);
});

test('title limits count Unicode characters and body remains optional', () => {
  const draft = announcementDraft(null);
  draft.title = '😀'.repeat(256);
  const valid = validateAnnouncementDraft(draft);
  assert.ok(valid.kind === 'valid');
  assert.equal(valid.payload.bodyMarkdown, null);
  draft.title += '字';
  const invalid = validateAnnouncementDraft(draft);
  assert.ok(invalid.kind === 'invalid');
  assert.ok(invalid.fieldErrors.title);
  draft.title = '   ';
  assert.equal(validateAnnouncementDraft(draft).kind, 'invalid');
});

test('priority accepts integer bounds, rejects empty or fractions and is zero for banners', () => {
  const draft = announcementDraft(null);
  draft.title = '公告';
  for (const priority of [-2147483648, 2147483647]) {
    draft.priority = priority;
    assert.equal(validateAnnouncementDraft(draft).kind, 'valid');
  }
  for (const priority of [undefined, NaN, 0.5, -2147483649, 2147483648]) {
    draft.priority = priority;
    const result = validateAnnouncementDraft(draft);
    assert.ok(result.kind === 'invalid');
    assert.ok(result.fieldErrors.priority);
  }
  draft.kind = 'BANNER';
  const banner = validateAnnouncementDraft(draft);
  assert.ok(banner.kind === 'valid');
  assert.equal(banner.payload.priority, 0);
});

test('link validation follows the selected action and ignores inactive fields', () => {
  const draft = announcementDraft(null);
  draft.title = '公告';
  draft.actionLabel = '查看项目';
  draft.actionTarget = '/about?source=announcement#intro';
  draft.actionType = 'INTERNAL';
  assert.equal(validateAnnouncementDraft(draft).kind, 'valid');
  for (const target of ['about', '//evil.test', '/\\evil.test', '/bad path']) {
    draft.actionTarget = target;
    assert.equal(validateAnnouncementDraft(draft).kind, 'invalid', target);
  }
  draft.actionType = 'EXTERNAL';
  for (const target of ['https://example.com/news', 'http://example.com/news']) {
    draft.actionTarget = target;
    assert.equal(validateAnnouncementDraft(draft).kind, 'valid');
  }
  for (const target of [
    '/about',
    'example.com',
    'https://user:pass@example.com',
    `${'java'}script:1`,
  ]) {
    draft.actionTarget = target;
    assert.equal(validateAnnouncementDraft(draft).kind, 'invalid', target);
  }
  draft.actionType = 'NONE';
  draft.actionLabel = '';
  const none = validateAnnouncementDraft(draft);
  assert.ok(none.kind === 'valid');
  assert.equal(none.payload.actionExternalUrl, null);
});

test('time errors identify missing, malformed and reversed visibility boundaries', () => {
  const draft = announcementDraft(null);
  draft.title = '公告';
  draft.endsAt = '2026-10-05T12:00:00';
  const missing = validateAnnouncementDraft(draft);
  assert.ok(missing.kind === 'invalid');
  assert.ok(missing.fieldErrors.startsAt);
  for (const startsAt of ['invalid', '2026-02-30T12:00:00', '2026-10-05']) {
    draft.startsAt = startsAt;
    assert.equal(validateAnnouncementDraft(draft).kind, 'invalid', startsAt);
  }
  draft.startsAt = draft.endsAt;
  const equal = validateAnnouncementDraft(draft);
  assert.ok(equal.kind === 'invalid');
  assert.ok(equal.fieldErrors.endsAt);
  draft.endsAt = '2026-10-06T12:00:00';
  assert.equal(validateAnnouncementDraft(draft).kind, 'valid');
});

test('publishing validates persisted dates against the current time', () => {
  const now = Date.parse('2026-10-03T12:00:00Z');
  assert.ok(validateAnnouncementPublish({ startsAt: null, endsAt: null }, true, now).startsAt);
  assert.ok(
    validateAnnouncementPublish({ startsAt: '2026-10-03T12:00:00Z', endsAt: null }, true, now)
      .startsAt,
  );
  assert.ok(
    validateAnnouncementPublish({ startsAt: null, endsAt: '2026-10-03T12:00:00Z' }, false, now)
      .endsAt,
  );
  assert.deepEqual(
    validateAnnouncementPublish({ startsAt: '2026-10-04T12:00:00Z', endsAt: null }, true, now),
    {},
  );
});

test('local date display uses the browser canonical minute format without dropping nonzero seconds', () => {
  assert.equal(toLocalDateTime('2026-10-03T08:00:00.123456Z').length, 16);
  assert.equal(toLocalDateTime('2026-10-03T08:00:30.123456Z').length, 19);
});
