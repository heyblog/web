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

import { inspectionFixture, jobFixture, restorationFixture } from './database-backup.fixture.ts';

test('inspection parsing validates the dataset inventory and restoration readiness', () => {
  // Given
  const cases = [
    { ...inspectionFixture, datasets: inspectionFixture.datasets.slice(1) },
    {
      ...inspectionFixture,
      datasets: inspectionFixture.datasets.map(() => inspectionFixture.datasets[0]),
    },
    {
      ...inspectionFixture,
      datasets: inspectionFixture.datasets.map((dataset, index) =>
        index === 0 ? { ...dataset, name: 'directory.unexpected_dataset' } : dataset,
      ),
    },
    { ...inspectionFixture, can_restore: true, target_ready: false },
    { ...inspectionFixture, issues: [{ code: 'target_not_empty' }] },
    { ...inspectionFixture, graph: { vertices: -1, edges: 2 } },
    { ...inspectionFixture, schema_version: 1 },
    { ...inspectionFixture, schema_version: 2 },
  ];
  assert.equal(inspectionFixture.datasets.length, 33);
  // When / Then
  assert.deepEqual(parseBackupInspection(inspectionFixture), inspectionFixture);
  const current = { ...inspectionFixture, schema_version: 4 };
  assert.deepEqual(parseBackupInspection(current), current);
  for (const input of cases) assert.equal(parseBackupInspection(input), null);
  assert.equal(parseBackupRestoration({ ...restorationFixture, admin_mapping: {} }), null);
});

test('restore upload binds the reviewed checksum and confirmation to the original file', async () => {
  // Given
  const file = new File(['{"synthetic":true}'], 'fixture.json', { type: 'application/json' });
  let calls = 0;
  const transport = createBackupTransport(async (input, init) => {
    calls += 1;
    assert.equal(input, '/api/web/management/database-backup/restore');
    assert.equal(init?.method, 'POST');
    assert.equal(init?.credentials, 'same-origin');
    assert.equal(init?.cache, 'no-store');
    assert.ok(init?.body instanceof FormData);
    assert.equal(init.body.get('sha256'), inspectionFixture.sha256);
    assert.equal(init.body.get('confirmation'), 'RESTORE');
    const uploaded = init.body.get('file');
    assert.ok(uploaded instanceof File);
    assert.equal(await uploaded.text(), await file.text());
    return Response.json(jobFixture, { status: 202 });
  });
  // When
  const result = await transport.restore(
    file,
    inspectionFixture.sha256,
    new AbortController().signal,
  );
  // Then
  assert.deepEqual(result, jobFixture);
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

test('job transport queries current and individual tasks and sends cancellation once', async () => {
  const calls: string[] = [];
  const transport = createBackupTransport(async (input, init) => {
    calls.push(`${init?.method} ${String(input)}`);
    assert.equal(init?.credentials, 'same-origin');
    return Response.json(String(input).endsWith('/current') ? { job: jobFixture } : jobFixture);
  });
  const signal = new AbortController().signal;
  assert.deepEqual(await transport.current(signal), { job: jobFixture });
  assert.deepEqual(await transport.job(jobFixture.id, signal), jobFixture);
  await transport.cancel(jobFixture.id, signal);
  assert.deepEqual(calls, [
    'GET /api/web/management/database-backup/restore-jobs/current',
    `GET /api/web/management/database-backup/restore-jobs/${jobFixture.id}`,
    `POST /api/web/management/database-backup/restore-jobs/${jobFixture.id}/cancel`,
  ]);
});

test('restore server failures and malformed acceptance responses require task recovery', async () => {
  for (const response of [
    Response.json({ code: 'bad_gateway' }, { status: 502 }),
    Response.json({ accepted: true }, { status: 202 }),
  ]) {
    let calls = 0;
    const transport = createBackupTransport(async () => {
      calls += 1;
      return response;
    });
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
  }
});
