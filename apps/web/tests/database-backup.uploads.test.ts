import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import test from 'node:test';

import {
  hashBackupFile,
  putBackupPart,
} from '../src/api/database-backup/database-backup.transfer.browser.ts';
import {
  parseBackupUpload,
  parseCurrentBackupUpload,
  parsePartURL,
  parseUploadOrigin,
} from '../src/api/database-backup/database-backup.uploads.ts';

import { inspectionFixture, uploadFixture, uploadsFixture } from './database-backup.fixture.ts';

test('upload boundary rejects conflicting manifests, invalid progress and untrusted origins', () => {
  assert.deepEqual(parseBackupUpload(uploadFixture), uploadFixture);
  for (const value of [
    { ...uploadFixture, part_sha256: [] },
    { ...uploadFixture, part_sha256: ['bad'] },
    { ...uploadFixture, uploaded_parts: [0] },
    { ...uploadFixture, uploaded_parts: [1, 1] },
    { ...uploadFixture, uploaded_bytes: 3 },
    { ...uploadFixture, prepared_bytes: -1 },
    { ...uploadFixture, state: 'unexpected' },
    { ...uploadFixture, inspection: { ...inspectionFixture, sha256: 'f'.repeat(64) } },
  ])
    assert.equal(parseBackupUpload(value), null);
  for (const origin of [
    'http://bucket.cos.ap-singapore.myqcloud.com',
    'https://user@bucket.cos.ap-singapore.myqcloud.com',
    'https://bucket.cos.ap-singapore.myqcloud.com/path',
    'https://attacker.test',
    'https://bucket.cos.ap-singapore.myqcloud.com.evil.test',
  ]) {
    assert.equal(parseUploadOrigin(origin), null);
    assert.equal(
      parseCurrentBackupUpload({
        ...uploadsFixture,
        settings: { ...uploadsFixture.settings, upload_origin: origin },
      }),
      null,
    );
  }
  assert.deepEqual(parseCurrentBackupUpload(uploadsFixture), uploadsFixture);
  assert.equal(
    parsePartURL({
      url: 'https://attacker.test/upload?secret=synthetic',
      expires_at: uploadFixture.expires_at,
    }),
    null,
  );
});

test('local file preparation hashes each complete part and honours cancellation', async () => {
  const data = new Uint8Array(4 * 1024 * 1024 + 3).fill(7);
  const file = new File([data], 'synthetic.json');
  const hashes = await hashBackupFile(file, new AbortController().signal);
  assert.deepEqual(hashes, [
    createHash('sha256')
      .update(data.subarray(0, 4 * 1024 * 1024))
      .digest('hex'),
    createHash('sha256')
      .update(data.subarray(4 * 1024 * 1024))
      .digest('hex'),
  ]);
  const controller = new AbortController();
  controller.abort();
  await assert.rejects(hashBackupFile(file, controller.signal), { message: 'cancelled' });
});

test('upload settings require consistent capability, transport and origin', () => {
  // Given
  const base = uploadsFixture.settings;
  const direct = { ...base, transport: 'direct', upload_origin: '' };
  const disabled = { ...base, transport: 'disabled', enabled: false, upload_origin: '' };
  // When / Then
  for (const settings of [direct, disabled])
    assert.deepEqual(parseCurrentBackupUpload({ upload: null, settings }), {
      upload: null,
      settings,
    });
  for (const settings of [
    { ...base, transport: undefined },
    { ...base, transport: 'unexpected' },
    { ...base, enabled: false },
    { ...direct, enabled: false },
    { ...direct, upload_origin: base.upload_origin },
    { ...disabled, enabled: true },
    { ...disabled, upload_origin: base.upload_origin },
  ])
    assert.equal(parseCurrentBackupUpload({ upload: null, settings }), null);
});

test('signed PUT never sends file bytes to an origin different from settings', async () => {
  await assert.rejects(
    putBackupPart({
      transport: 'cos',
      url: 'https://other.cos.ap-singapore.myqcloud.com/private',
      origin: uploadsFixture.settings.upload_origin,
      body: new Blob(['synthetic']),
      signal: new AbortController().signal,
      progress() {},
    }),
    { message: 'invalid_response' },
  );
});
