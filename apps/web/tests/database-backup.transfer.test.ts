import assert from 'node:assert/strict';
import test, { type TestContext } from 'node:test';

import { createBackupTransport } from '../src/api/database-backup/database-backup.browser.ts';
import {
  putBackupPart,
  transferBackupFile,
} from '../src/api/database-backup/database-backup.transfer.browser.ts';
import { backupPartSize } from '../src/api/database-backup/database-backup.uploads.ts';

import { uploadFixture, uploadsFixture } from './database-backup.fixture.ts';

class TestPartRequest {
  readonly headers = new Headers();
  readonly upload: { onprogress: ((event: { readonly loaded: number }) => void) | null } = {
    onprogress: null,
  };
  url = '';
  method = '';
  withCredentials = false;
  timeout = 0;
  status = 0;
  responseText = '';
  aborted = false;
  body: Blob | undefined;
  onload: (() => void) | null = null;
  onerror: (() => void) | null = null;
  ontimeout: (() => void) | null = null;
  onabort: (() => void) | null = null;
  private readonly sent: (request: TestPartRequest) => void;
  constructor(sent: (request: TestPartRequest) => void) {
    this.sent = sent;
  }
  open(method: string, url: string) {
    this.method = method;
    this.url = url;
  }
  setRequestHeader(name: string, value: string) {
    this.headers.set(name, value);
  }
  send(body: Blob) {
    this.body = body;
    this.sent(this);
  }
  abort() {
    this.aborted = true;
    this.onabort?.();
  }
}

function installPartRequests(context: TestContext, sent: (request: TestPartRequest) => void) {
  const descriptor = Object.getOwnPropertyDescriptor(globalThis, 'XMLHttpRequest');
  const requests: TestPartRequest[] = [];
  Object.defineProperty(globalThis, 'XMLHttpRequest', {
    configurable: true,
    value: class extends TestPartRequest {
      constructor() {
        super(sent);
        requests.push(this);
      }
    },
  });
  context.after(() => {
    if (descriptor) Object.defineProperty(globalThis, 'XMLHttpRequest', descriptor);
    else Reflect.deleteProperty(globalThis, 'XMLHttpRequest');
  });
  return requests;
}

const directSettings = {
  ...uploadsFixture.settings,
  transport: 'direct',
  upload_origin: '',
} as const;

test('direct upload resumes missing parts with two concurrent same-origin PUTs and progress', async (t) => {
  // Given
  const file = new File([new Uint8Array(backupPartSize * 2 + 3)], 'synthetic.json');
  const session = {
    ...uploadFixture,
    size: file.size,
    part_sha256: ['0'.repeat(64), '1'.repeat(64), '2'.repeat(64)],
    uploaded_parts: [1],
  };
  let active = 0;
  let maximum = 0;
  const requests = installPartRequests(t, (request) => {
    active += 1;
    maximum = Math.max(maximum, active);
    request.upload.onprogress?.({ loaded: request.body?.size ?? 0 });
    queueMicrotask(() => {
      active -= 1;
      request.status = 204;
      request.onload?.();
    });
  });
  const progress: number[] = [];
  // When
  await transferBackupFile({
    session,
    file,
    settings: directSettings,
    signal: new AbortController().signal,
    transport: createBackupTransport(async () => {
      throw new TypeError('Direct transfer must not request signed URLs');
    }),
    progress: (value) => progress.push(value.bytes),
  });
  // Then
  assert.equal(maximum, 2);
  assert.deepEqual(
    requests.map((request) => [request.method, request.url, request.body?.size]),
    [
      ['PUT', `/api/web/management/database-backup/uploads/${session.id}/parts/2`, backupPartSize],
      ['PUT', `/api/web/management/database-backup/uploads/${session.id}/parts/3`, 3],
    ],
  );
  assert.ok(requests.every((request) => request.withCredentials));
  assert.ok(
    requests.every((request) => request.headers.get('Content-Type') === 'application/octet-stream'),
  );
  assert.equal(progress[0], backupPartSize);
  assert.equal(progress.at(-1), file.size);
});

test('COS upload retains signed external PUTs without cross-origin credentials', async (t) => {
  // Given
  const requests = installPartRequests(t, (request) => {
    queueMicrotask(() => {
      request.status = 200;
      request.onload?.();
    });
  });
  const signed = `${uploadsFixture.settings.upload_origin}/private?uploadId=synthetic`;
  let signedRequests = 0;
  // When
  await transferBackupFile({
    session: { ...uploadFixture, uploaded_parts: [] },
    file: new File(['{}'], 'synthetic.json'),
    settings: uploadsFixture.settings,
    signal: new AbortController().signal,
    transport: createBackupTransport(async () => {
      signedRequests += 1;
      return Response.json({ url: signed, expires_at: uploadFixture.expires_at });
    }),
    progress() {},
  });
  // Then
  assert.equal(signedRequests, 1);
  assert.equal(requests[0]?.url, signed);
  assert.equal(requests[0]?.withCredentials, false);
});

test('pausing a direct transfer aborts both outstanding part requests', async (t) => {
  // Given
  let launched = () => {};
  const ready = new Promise<void>((resolve) => {
    launched = resolve;
  });
  let count = 0;
  const requests = installPartRequests(t, () => {
    if (++count === 2) launched();
  });
  const controller = new AbortController();
  const file = new File([new Uint8Array(backupPartSize + 1)], 'synthetic.json');
  const transfer = transferBackupFile({
    session: {
      ...uploadFixture,
      size: file.size,
      part_sha256: ['0'.repeat(64), '1'.repeat(64)],
      uploaded_parts: [],
    },
    file,
    settings: directSettings,
    signal: controller.signal,
    transport: createBackupTransport(),
    progress() {},
  });
  await ready;
  // When
  controller.abort();
  // Then
  await assert.rejects(transfer, { code: 'cancelled' });
  assert.equal(requests.length, 2);
  assert.ok(requests.every((request) => request.aborted));
});

test('an uncertain direct PUT checks accepted parts before retransmitting bytes', async (t) => {
  // Given
  const requests = installPartRequests(t, (request) => {
    queueMicrotask(() => request.onerror?.());
  });
  let reconciled = 0;
  // When
  await transferBackupFile({
    session: { ...uploadFixture, uploaded_parts: [] },
    file: new File(['{}'], 'synthetic.json'),
    settings: directSettings,
    signal: new AbortController().signal,
    transport: createBackupTransport(async (input, init) => {
      assert.ok(String(input).endsWith(`/uploads/${uploadFixture.id}`));
      assert.equal(init?.method, 'GET');
      reconciled += 1;
      return Response.json(uploadFixture);
    }),
    progress() {},
  });
  // Then
  assert.equal(reconciled, 1);
  assert.equal(requests.length, 1);
});

test('direct part acceptance requires the API no-content response', async (t) => {
  // Given
  installPartRequests(t, (request) => {
    queueMicrotask(() => {
      request.status = 200;
      request.onload?.();
    });
  });
  // When / Then
  await assert.rejects(
    putBackupPart({
      transport: 'direct',
      id: uploadFixture.id,
      number: 1,
      body: new Blob(['{}']),
      signal: new AbortController().signal,
      progress() {},
    }),
    { code: 'upload_failed', status: 200 },
  );
});
