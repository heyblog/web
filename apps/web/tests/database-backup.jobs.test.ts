import assert from 'node:assert/strict';
import test from 'node:test';

import {
  parseCurrentRestoreJob,
  parseRestoreJob,
} from '../src/api/database-backup/database-backup.jobs.ts';

import { jobFixture, restorationFixture } from './database-backup.fixture.ts';

test('job parser accepts completed results only with a valid committed restoration', () => {
  assert.deepEqual(parseRestoreJob(jobFixture), jobFixture);
  const success = { ...jobFixture, status: 'succeeded', result: restorationFixture };
  assert.deepEqual(parseRestoreJob(success), success);
  for (const value of [
    { ...jobFixture, status: 'succeeded' },
    { ...jobFixture, result: restorationFixture },
    { ...success, error_code: 'failed' },
    { ...jobFixture, status: 'failed', error_code: 'private SQL text' },
    { ...jobFixture, progress: { ...jobFixture.progress, completed: 2 } },
    { ...jobFixture, progress: { ...jobFixture.progress, dataset: 'unexpected.table' } },
  ])
    assert.equal(parseRestoreJob(value), null);
  assert.deepEqual(parseCurrentRestoreJob({ job: null }), { job: null });
  assert.equal(parseCurrentRestoreJob({}), null);
  const cancelled = { ...jobFixture, status: 'cancelled', error_code: 'restore_cancelled' };
  assert.deepEqual(parseRestoreJob(cancelled), cancelled);
});
