import assert from 'node:assert/strict';
import test from 'node:test';

import {
  dismissAnnouncement,
  isAnnouncementDismissed,
} from '../src/application/announcements/announcement-dismissal.browser.ts';

test('dismissals persist independently by announcement ID across reads', () => {
  const values = new Map<string, string>();
  const storage = () => ({
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => {
      values.set(key, value);
    },
  });
  assert.equal(isAnnouncementDismissed('first', storage), false);
  dismissAnnouncement('first', storage);
  assert.equal(isAnnouncementDismissed('first', storage), true);
  assert.equal(isAnnouncementDismissed('second', storage), false);
  dismissAnnouncement('second', storage);
  assert.equal(isAnnouncementDismissed('first', storage), true);
  assert.equal(isAnnouncementDismissed('second', storage), true);
});

test('denied storage access does not prevent display or closing', () => {
  const storage = () => {
    throw new DOMException('Denied', 'SecurityError');
  };
  assert.equal(isAnnouncementDismissed('first', storage), false);
  assert.doesNotThrow(() => dismissAnnouncement('first', storage));
});

test('storage method failures are best effort', () => {
  const storage = () => ({
    getItem: () => {
      throw new Error('Read unavailable');
    },
    setItem: () => {
      throw new DOMException('Full', 'QuotaExceededError');
    },
  });
  assert.equal(isAnnouncementDismissed('first', storage), false);
  assert.doesNotThrow(() => dismissAnnouncement('first', storage));
});
