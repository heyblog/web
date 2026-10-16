import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import test from 'node:test';

import { createBackupTransport } from '../src/api/database-backup/database-backup.browser.ts';
import { backupMessage } from '../src/application/database-backup/database-backup.messages.ts';

import { createTestBackupController } from './database-backup.controller-loader.ts';
import { jobFixture, restorationFixture, uploadsFixture } from './database-backup.fixture.ts';

test('reselection rejects a different file before requesting COS upload URLs', async () => {
  const calls: string[] = [];
  const session = {
    ...uploadsFixture.upload,
    state: 'uploading',
    inspection: undefined,
    sha256: undefined,
  };
  const controller = createTestBackupController(
    createBackupTransport(async (input) => {
      calls.push(String(input));
      return Response.json(
        String(input).includes('/uploads/')
          ? { ...uploadsFixture, upload: session }
          : { job: null },
      );
    }),
    uploadsFixture,
  );
  await controller.refresh();
  controller.upload.select(new File(['{}'], 'same-name.json'));
  await controller.startUpload();
  assert.equal(controller.upload.error, backupMessage('different_file'));
  assert.equal(
    calls.some((path) => path.includes('/parts/')),
    false,
  );
  controller.dispose();
});

test('an already uploaded file is prepared without sending bytes a second time', async () => {
  const hash = createHash('sha256').update('{}').digest('hex');
  const session = {
    ...uploadsFixture.upload,
    state: 'uploading',
    inspection: undefined,
    sha256: undefined,
    part_sha256: [hash],
  };
  const calls: string[] = [];
  const controller = createTestBackupController(
    createBackupTransport(async (input) => {
      const path = String(input);
      calls.push(path);
      if (path.endsWith('/uploads/current'))
        return Response.json({ ...uploadsFixture, upload: session });
      if (path.endsWith('/restore-jobs/current')) return Response.json({ job: null });
      return Response.json({
        ...session,
        state: path.endsWith('/complete') ? 'preparing' : 'uploading',
      });
    }),
    uploadsFixture,
  );
  await controller.refresh();
  controller.upload.select(new File(['{}'], 'renamed.json'));
  await controller.startUpload();
  assert.equal(controller.upload.session?.state, 'preparing');
  assert.equal(calls.filter((path) => path.endsWith('/complete')).length, 1);
  assert.equal(
    calls.some((path) => path.includes('/parts/')),
    false,
  );
  controller.dispose();
});

test('successful restore remains successful while temporary file cleanup is pending', async () => {
  const controller = createTestBackupController(
    createBackupTransport(async (input) =>
      Response.json(
        String(input).includes('/uploads/')
          ? {
              ...uploadsFixture,
              upload: { ...uploadsFixture.upload, state: 'succeeded', cleanup_status: 'pending' },
            }
          : { job: { ...jobFixture, status: 'succeeded', result: restorationFixture } },
      ),
    ),
  );
  await controller.refresh();
  assert.deepEqual(controller.restoration, restorationFixture);
  assert.equal(controller.upload.session?.cleanup_status, 'pending');
  assert.equal(controller.acceptanceUnknown, false);
  assert.equal(controller.upload.inspection, null);
  controller.dispose();
});

test('missing or changed SSR upload settings block upload until page reload', async () => {
  for (const initial of [
    null,
    {
      ...uploadsFixture,
      settings: {
        ...uploadsFixture.settings,
        upload_origin: 'https://other-123.cos.ap-singapore.myqcloud.com',
      },
    },
    {
      ...uploadsFixture,
      settings: {
        ...uploadsFixture.settings,
        transport: 'direct' as const,
        upload_origin: '' as const,
      },
    },
  ]) {
    const controller = createTestBackupController(
      createBackupTransport(async (input) =>
        Response.json(String(input).includes('/uploads/') ? uploadsFixture : { job: null }),
      ),
      initial,
    );
    await controller.refresh();
    assert.equal(controller.upload.settings?.enabled, false);
    assert.equal(controller.upload.error, backupMessage('upload_settings_changed'));
    controller.upload.select(new File(['{}'], 'synthetic.json'));
    await controller.startUpload();
    assert.equal(controller.upload.busy, null);
    controller.dispose();
  }
});

test('a development direct transport survives refresh and resumes an already uploaded file', async () => {
  // Given
  const hash = createHash('sha256').update('{}').digest('hex');
  const initial = {
    ...uploadsFixture,
    settings: {
      ...uploadsFixture.settings,
      transport: 'direct' as const,
      upload_origin: '' as const,
    },
    upload: {
      ...uploadsFixture.upload,
      state: 'uploading' as const,
      inspection: undefined,
      sha256: undefined,
      part_sha256: [hash],
    },
  };
  const controller = createTestBackupController(
    createBackupTransport(async (input) => {
      const path = String(input);
      if (path.endsWith('/uploads/current')) return Response.json(initial);
      if (path.endsWith('/restore-jobs/current')) return Response.json({ job: null });
      return Response.json({
        ...initial.upload,
        state: path.endsWith('/complete') ? 'preparing' : 'uploading',
      });
    }),
    initial,
  );
  // When
  await controller.refresh();
  controller.upload.select(new File(['{}'], 'synthetic.json'));
  await controller.startUpload();
  // Then
  assert.equal(controller.upload.settings?.transport, 'direct');
  assert.equal(controller.upload.settings?.enabled, true);
  assert.equal(controller.upload.session?.state, 'preparing');
  assert.equal(controller.upload.error, null);
  controller.dispose();
});

test('enabling direct upload after SSR requires reload even though both origins are empty', async () => {
  // Given
  const disabled = {
    ...uploadsFixture,
    settings: {
      ...uploadsFixture.settings,
      transport: 'disabled' as const,
      enabled: false as const,
      upload_origin: '' as const,
    },
  };
  const current = {
    ...uploadsFixture,
    settings: {
      ...uploadsFixture.settings,
      transport: 'direct' as const,
      upload_origin: '' as const,
    },
  };
  const controller = createTestBackupController(
    createBackupTransport(async (input) =>
      Response.json(String(input).includes('/uploads/') ? current : { job: null }),
    ),
    disabled,
  );
  // When
  await controller.refresh();
  // Then
  assert.equal(controller.upload.settings?.enabled, false);
  assert.equal(controller.upload.error, backupMessage('upload_settings_changed'));
  controller.dispose();
});

test('terminal upload does not expose stale inspection from SSR', () => {
  const controller = createTestBackupController(undefined, {
    ...uploadsFixture,
    upload: { ...uploadsFixture.upload, state: 'succeeded', cleanup_status: 'completed' },
  });
  assert.equal(controller.upload.inspection, null);
  controller.dispose();
});

test('an unknown upload retains recovery uncertainty when no current job exists', async () => {
  const controller = createTestBackupController(
    createBackupTransport(async (input) =>
      Response.json(
        String(input).includes('/uploads/')
          ? { ...uploadsFixture, upload: { ...uploadsFixture.upload, state: 'unknown' } }
          : { job: null },
      ),
    ),
    uploadsFixture,
  );
  await controller.refresh();
  assert.equal(controller.acceptanceUnknown, true);
  assert.equal(controller.upload.inspection, null);
  assert.equal(controller.error, backupMessage('restore_outcome_unknown'));
  controller.dispose();
});

test('a create conflict recovers the session created by another tab', async () => {
  let reads = 0;
  const controller = createTestBackupController(
    createBackupTransport(async (input) => {
      const path = String(input);
      if (path.endsWith('/uploads/current'))
        return Response.json({
          ...uploadsFixture,
          upload:
            ++reads === 1
              ? null
              : {
                  ...uploadsFixture.upload,
                  state: 'uploading',
                  inspection: undefined,
                  sha256: undefined,
                },
        });
      if (path.endsWith('/restore-jobs/current')) return Response.json({ job: null });
      return Response.json({ code: 'database_backup_busy' }, { status: 409 });
    }),
    { ...uploadsFixture, upload: null },
  );
  await controller.refresh();
  controller.upload.select(new File(['{}'], 'synthetic.json'));
  await controller.startUpload();
  assert.equal(reads, 2);
  assert.equal(controller.upload.session?.id, uploadsFixture.upload.id);
  assert.equal(controller.upload.session?.state, 'uploading');
  controller.dispose();
});
