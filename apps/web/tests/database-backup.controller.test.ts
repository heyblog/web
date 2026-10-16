import assert from 'node:assert/strict';
import test from 'node:test';

import { createBackupTransport } from '../src/api/database-backup/database-backup.browser.ts';
import { backupMessage } from '../src/application/database-backup/database-backup.messages.ts';

import { createTestBackupController } from './database-backup.controller-loader.ts';
import {
  inspectionFixture,
  jobFixture,
  restorationFixture,
  uploadsFixture,
} from './database-backup.fixture.ts';

function testTransport(fetcher: typeof fetch) {
  return createBackupTransport(async (input, init) => {
    if (String(input).includes('/uploads/')) return Response.json(uploadsFixture);
    return fetcher(input, init);
  });
}

test('an unknown outcome retains uncertainty even when its error code describes a failed restore', async () => {
  const controller = createTestBackupController(
    testTransport(async () =>
      Response.json({
        job: { ...jobFixture, status: 'unknown', error_code: 'database_backup_restore_failed' },
      }),
    ),
  );
  await controller.refresh();
  assert.equal(controller.error, backupMessage('restore_outcome_unknown'));
  assert.equal(controller.restoration, null);
  assert.equal(controller.acceptanceUnknown, true);
  controller.dispose();
});

test('leaving an accepted restore stops polling without sending cancellation', async () => {
  const calls: string[] = [];
  const transport = testTransport(async (input, init) => {
    calls.push(`${init?.method} ${String(input)}`);
    if (String(input).endsWith('/current')) return Response.json({ job: null });
    if (String(input).endsWith('/inspect')) return Response.json(inspectionFixture);
    return Response.json(jobFixture, { status: 202 });
  });
  const controller = createTestBackupController(transport);
  await controller.refresh();
  controller.upload.select(new File(['{}'], 'fixture.json'));
  await controller.upload.inspect();
  await controller.restore();
  assert.equal(controller.active, true);
  controller.dispose();
  assert.equal(calls.filter((call) => call.endsWith('/restore')).length, 1);
  assert.equal(
    calls.some((call) => call.endsWith('/cancel')),
    false,
  );
});
test('page reentry retrieves a completed task without uploading a file', async () => {
  const result = { ...jobFixture, status: 'succeeded', result: restorationFixture };
  const controller = createTestBackupController(
    testTransport(async (input) => {
      assert.ok(String(input).endsWith('/current'));
      return Response.json({ job: result });
    }),
  );
  await controller.refresh();
  assert.deepEqual(controller.restoration, restorationFixture);
  assert.equal(controller.blocked, false);
  controller.dispose();
});
test('uncertain restore acceptance is recovered through current task instead of repeating upload', async () => {
  let uploads = 0;
  let currentReads = 0;
  const controller = createTestBackupController(
    testTransport(async (input) => {
      if (String(input).endsWith('/current')) {
        currentReads += 1;
        return Response.json({ job: currentReads === 1 ? null : jobFixture });
      }
      if (String(input).endsWith('/inspect')) return Response.json(inspectionFixture);
      uploads += 1;
      throw new TypeError('connection interrupted');
    }),
  );
  await controller.refresh();
  controller.upload.select(new File(['{}'], 'fixture.json'));
  await controller.upload.inspect();
  await controller.restore();
  assert.equal(uploads, 1);
  assert.equal(currentReads, 2);
  assert.equal(controller.active, true);
  assert.equal(controller.acceptanceUnknown, false);
  controller.dispose();
});

test('an older completed task cannot resolve uncertainty about a newly uploaded restore', async () => {
  const previous = { ...jobFixture, status: 'succeeded', result: restorationFixture };
  const controller = createTestBackupController(
    testTransport(async (input) => {
      if (String(input).endsWith('/current')) return Response.json({ job: previous });
      if (String(input).endsWith('/inspect')) return Response.json(inspectionFixture);
      throw new TypeError('connection interrupted');
    }),
  );
  await controller.refresh();
  controller.upload.select(new File(['{}'], 'fixture.json'));
  await controller.upload.inspect();
  await controller.restore();
  assert.equal(controller.acceptanceUnknown, true);
  assert.equal(controller.restoration, null);
  assert.equal(controller.blocked, true);
  controller.dispose();
});

test('a recovered result with a different administrator mapping does not display success', async () => {
  let reads = 0;
  const controller = createTestBackupController(
    testTransport(async (input) => {
      if (String(input).endsWith('/current')) {
        reads += 1;
        return Response.json({
          job:
            reads === 1
              ? null
              : {
                  ...jobFixture,
                  status: 'succeeded',
                  result: {
                    ...restorationFixture,
                    admin_mapping: {
                      ...restorationFixture.admin_mapping,
                      target_id: jobFixture.id,
                    },
                  },
                },
        });
      }
      if (String(input).endsWith('/inspect')) return Response.json(inspectionFixture);
      throw new TypeError('connection interrupted');
    }),
  );
  await controller.refresh();
  controller.upload.select(new File(['{}'], 'fixture.json'));
  await controller.upload.inspect();
  await controller.restore();
  assert.equal(controller.restoration, null);
  assert.equal(controller.acceptanceUnknown, true);
  controller.dispose();
});

test('inspection conflicts query and resume an active restore without repeating the upload', async () => {
  // Given
  const calls: string[] = [];
  let reads = 0;
  const controller = createTestBackupController(
    testTransport(async (input) => {
      calls.push(String(input));
      if (String(input).endsWith('/current'))
        return Response.json({ job: ++reads === 1 ? null : jobFixture });
      return Response.json({ code: 'database_backup_busy' }, { status: 409 });
    }),
  );
  await controller.refresh();
  controller.upload.select(new File(['{}'], 'fixture.json'));
  try {
    // When
    await controller.upload.inspect();
    // Then
    assert.equal(controller.active, true);
    assert.equal(reads, 2);
    assert.equal(calls.filter((call) => call.endsWith('/inspect')).length, 1);
  } finally {
    controller.dispose();
  }
});

test('a null current restore preserves the inspection conflict instead of implying an idle service', async () => {
  // Given
  let reads = 0;
  const controller = createTestBackupController(
    testTransport(async (input) => {
      if (String(input).endsWith('/current')) {
        reads++;
        return Response.json({ job: null });
      }
      return Response.json({ code: 'database_backup_busy' }, { status: 409 });
    }),
  );
  await controller.refresh();
  controller.upload.select(new File(['{}'], 'fixture.json'));
  try {
    // When
    await controller.upload.inspect();
    // Then
    assert.equal(reads, 2);
    assert.equal(controller.job, null);
    assert.equal(controller.upload.error, backupMessage('database_backup_busy'));
    assert.equal(controller.acceptanceUnknown, false);
  } finally {
    controller.dispose();
  }
});
