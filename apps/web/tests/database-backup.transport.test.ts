import assert from 'node:assert/strict';
import test from 'node:test';

import {
  BackupTransportError,
  createBackupTransport,
} from '../src/api/database-backup/database-backup.browser.ts';
import {
  parseBackupInspection,
  parseBackupRestoration,
} from '../src/api/database-backup/database-backup.types.ts';
import { managementNavigation } from '../src/application/management/navigation.ts';

import { inspectionFixture, restorationFixture } from './database-backup.fixture.ts';

test('inspection parsing rejects missing datasets, duplicate datasets and inconsistent readiness', () => {
  // Given
  const cases = [
    { ...inspectionFixture, datasets: inspectionFixture.datasets.slice(1) },
    {
      ...inspectionFixture,
      datasets: inspectionFixture.datasets.map(() => inspectionFixture.datasets[0]),
    },
    { ...inspectionFixture, can_restore: true, target_ready: false },
    { ...inspectionFixture, issues: [{ code: 'target_not_empty' }] },
    { ...inspectionFixture, graph: { vertices: -1, edges: 2 } },
    { ...inspectionFixture, schema_version: 1 },
    { ...inspectionFixture, schema_version: 2 },
  ];
  for (const name of [
    'directory.tag_labels',
    'directory.tag_slug_aliases',
    'directory.slug_generation_cache',
    'directory.slug_generation_jobs',
    'content.system_ai_settings',
  ]) {
    assert.equal(
      parseBackupInspection({
        ...inspectionFixture,
        datasets: [...inspectionFixture.datasets, { name, count: 0 }],
      }),
      null,
    );
  }
  assert.equal(inspectionFixture.datasets.length, 33);
  // When / Then
  assert.deepEqual(parseBackupInspection(inspectionFixture), inspectionFixture);
  for (const input of cases) assert.equal(parseBackupInspection(input), null);
  assert.equal(parseBackupRestoration({ ...restorationFixture, admin_mapping: {} }), null);
});

test('restore upload binds the reviewed checksum and confirmation to the original file', async () => {
  // Given
  const file = new File(['{"synthetic":true}'], 'fixture.json', { type: 'application/json' });
  let calls = 0;
  const transport = createBackupTransport(async (input, init) => {
    calls += 1;
    assert.equal(input, '/management/database-backup/restore');
    assert.equal(init?.method, 'POST');
    assert.equal(init?.credentials, 'same-origin');
    assert.equal(init?.cache, 'no-store');
    assert.ok(init?.body instanceof FormData);
    assert.equal(init.body.get('sha256'), inspectionFixture.sha256);
    assert.equal(init.body.get('confirmation'), 'RESTORE');
    const uploaded = init.body.get('file');
    assert.ok(uploaded instanceof File);
    assert.equal(await uploaded.text(), await file.text());
    return Response.json(restorationFixture);
  });
  // When
  const result = await transport.restore(
    file,
    inspectionFixture.sha256,
    new AbortController().signal,
  );
  // Then
  assert.deepEqual(result, restorationFixture);
  assert.equal(calls, 1);
});

test('failed restore is never automatically retried and preserves unknown outcome', async () => {
  // Given
  let calls = 0;
  const transport = createBackupTransport(async () => {
    calls += 1;
    throw new TypeError('network unavailable');
  });
  // When / Then
  await assert.rejects(
    transport.restore(
      new File(['{}'], 'fixture.json'),
      inspectionFixture.sha256,
      new AbortController().signal,
    ),
    (error: unknown) =>
      error instanceof BackupTransportError && error.code === 'restore_outcome_unknown',
  );
  assert.equal(calls, 1);
});

test('inspection failures never display server diagnostics and malformed successes are rejected', async () => {
  // Given
  const transport = createBackupTransport(async () =>
    Response.json(
      { code: 'database_backup_invalid', detail: 'private SQL diagnostics' },
      { status: 422 },
    ),
  );
  const invalid = createBackupTransport(async () =>
    Response.json({ ...inspectionFixture, datasets: [] }),
  );
  const file = new File(['{}'], 'fixture.json');
  // When / Then
  await assert.rejects(
    transport.inspect(file, new AbortController().signal),
    (error: unknown) =>
      error instanceof BackupTransportError && error.message === 'database_backup_invalid',
  );
  await assert.rejects(
    invalid.inspect(file, new AbortController().signal),
    (error: unknown) => error instanceof BackupTransportError && error.code === 'invalid_response',
  );
});

test('backup management navigation is available only to system administrators', () => {
  // Given / When
  const admin = managementNavigation({
    role: 'ADMIN',
    permissions: ['site.manage', 'user.manage'],
  });
  const system = managementNavigation({ role: 'SYS_ADMIN', permissions: [] });
  // Then
  assert.equal(
    admin.some((item) => item.href === '/management/database-backup'),
    false,
  );
  assert.equal(
    system.some((item) => item.href === '/management/database-backup'),
    true,
  );
});
