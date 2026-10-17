import assert from 'node:assert/strict';
import test from 'node:test';

import {
  ClaimProblem,
  createClaimsTransport,
  hashClaimAttachment,
  uploadClaimAttachment,
} from '../src/api/site-management/claims.browser.ts';
import {
  parseClaim,
  parseClaimContext,
  parseClaimPage,
  parseOwnership,
} from '../src/api/site-management/claims.types.ts';
import {
  claimAttachmentLimits,
  manualClaimValidation,
  validateClaimAttachment,
} from '../src/application/site-management/claim-attachments.ts';
import {
  claimDestination,
  claimLabel,
  claimMessage,
  verifiedClaimAddress,
} from '../src/application/site-management/claims.model.ts';
import { claimListQuery } from '../src/application/site-management/claims-query.ts';

import { claimFixture, contextFixture, ownershipFixture } from './site-claims.fixture.ts';

test('claim DTOs validate capabilities, expiration and paging before reaching the page', () => {
  assert.equal(parseClaim(claimFixture).site_name, 'Example');
  assert.throws(() => parseClaim({ ...claimFixture, can_manage: 'true' }));
  assert.throws(() => parseClaim({ ...claimFixture, expires_at: 'tomorrow' }));
  assert.throws(() => parseClaimPage({ items: [], has_more: 'true' }));
  assert.equal(parseClaimPage({ items: [claimFixture], has_more: true }).has_more, true);
  assert.deepEqual(parseClaimContext(contextFixture), contextFixture);
  assert.deepEqual(parseOwnership(ownershipFixture), ownershipFixture);
  assert.throws(() =>
    parseOwnership({ ...ownershipFixture, current: { ...ownershipFixture.current, revision: 0 } }),
  );
});
test('manual evidence validates Unicode length and requires a link or attachment', () => {
  assert.equal(manualClaimValidation('一'.repeat(19), '', []), 'manual_evidence_too_short');
  assert.equal(manualClaimValidation('🙂'.repeat(4000), 'https://example.test/', []), undefined);
  assert.equal(
    manualClaimValidation('一'.repeat(4001), 'https://example.test/', []),
    'invalid_evidence',
  );
  assert.equal(manualClaimValidation('🙂'.repeat(20), '', []), 'manual_attachment_required');
  assert.equal(
    manualClaimValidation('说明'.repeat(10), 'ftp://example.test', []),
    'invalid_evidence_url',
  );
  assert.equal(
    manualClaimValidation('说明'.repeat(10), '', [
      {
        id: 'attachment',
        filename: 'proof.pdf',
        content_type: 'application/pdf',
        size: 10,
        status: 'READY',
      },
    ]),
    undefined,
  );
});
test('claim attachments enforce file type, per-file and total limits', () => {
  const image = { name: 'proof.png', type: 'image/png', size: claimAttachmentLimits.maxImageBytes };
  assert.equal(validateClaimAttachment(image, 0, 0), undefined);
  assert.equal(
    validateClaimAttachment({ ...image, size: claimAttachmentLimits.maxImageBytes + 1 }, 0, 0),
    'manual_attachment_too_large',
  );
  assert.equal(
    validateClaimAttachment({ name: 'proof.exe', type: 'application/octet-stream', size: 1 }, 0, 0),
    'manual_attachment_type',
  );
  assert.equal(
    validateClaimAttachment(image, 0, claimAttachmentLimits.maxTotalBytes - image.size + 1),
    'manual_attachment_total_too_large',
  );
  assert.equal(
    validateClaimAttachment(image, claimAttachmentLimits.maxCount, 0),
    'manual_attachment_count',
  );
});
test('attachment transport initializes, directly uploads, completes and removes with bound IDs', async () => {
  const calls: Array<{ path: string; method: string; body: unknown }> = [];
  const transport = createClaimsTransport(async (path, options) => {
    calls.push({ path, method: options?.method ?? 'GET', body: options?.body });
    if (path === 'site-claim-attachments')
      return Response.json({
        attachment: {
          id: 'attachment',
          filename: 'proof.pdf',
          content_type: 'application/pdf',
          size: 4,
          status: 'UPLOADING',
        },
        upload_url: 'https://cos.example.test/upload',
        upload_headers: { 'X-Cos-Key': 'signed' },
        expires_at: '2099-10-02T00:00:00Z',
        max_size: 10,
        content_type: 'application/pdf',
      });
    if (path.endsWith('/complete'))
      return Response.json({
        attachment: {
          id: 'attachment',
          filename: 'proof.pdf',
          content_type: 'application/pdf',
          size: 4,
          status: 'READY',
        },
      });
    return Response.json({ success: true });
  });
  const signal = new AbortController().signal;
  const upload = await transport.initializeAttachment(
    {
      short_id: 'abc123456',
      address: 'https://example.test/',
      filename: 'proof.pdf',
      content_type: 'application/pdf',
      size: 4,
    },
    signal,
  );
  assert.equal(upload.attachment.id, 'attachment');
  const complete = await transport.completeAttachment(
    upload.attachment.id,
    { size: 4, content_type: 'application/pdf', sha256: 'a'.repeat(64) },
    signal,
  );
  assert.equal(complete.status, 'READY');
  await transport.removeAttachment('attachment', signal);
  assert.deepEqual(
    calls.map(({ path, method }) => `${method} ${path}`),
    [
      'POST site-claim-attachments',
      'POST site-claim-attachments/attachment/complete',
      'DELETE site-claim-attachments/attachment',
    ],
  );
  assert.deepEqual(calls[1]?.body, {
    size: 4,
    content_type: 'application/pdf',
    sha256: 'a'.repeat(64),
  });
});
test('signed attachment upload sends the file without browser credentials', async () => {
  let request: Request | undefined;
  const file = new Blob(['proof'], { type: 'application/pdf' });
  await uploadClaimAttachment(
    {
      upload_url: 'https://cos.example.test/upload',
      upload_headers: { 'X-Cos-Key': 'signed' },
      content_type: 'application/pdf',
    },
    file,
    new AbortController().signal,
    async (input, init) => {
      request = new Request(input, init);
      return new Response(null, { status: 204 });
    },
  );
  assert.equal(request?.method, 'PUT');
  assert.equal(request?.credentials, 'omit');
  assert.equal(await request?.blob().then((value) => value.size), file.size);
});
test('attachment hashing returns a stable SHA-256 digest', async () => {
  const digest = await hashClaimAttachment(new Blob(['proof']), new AbortController().signal);
  assert.equal(digest, 'c1cda26362828b69266512052b97cb3729e3b052e4ade47c0a1e3383defe73c7');
});
test('only current management permission and a verified unused address proof enable editor prefill', () => {
  const proof = {
    ...claimFixture,
    status: 'VERIFIED',
    can_manage: true,
    address: 'https://new.example.test/',
  } as const;
  assert.equal(verifiedClaimAddress(proof, proof.short_id), proof.address);
  assert.equal(
    claimDestination(proof),
    `/dashboard/sites/${proof.short_id}?verified_claim=${proof.id}#information`,
  );
  for (const invalid of [
    { ...proof, can_manage: false },
    { ...proof, consumed_at: proof.created_at },
    { ...proof, status: 'PENDING' as const },
    { ...proof, address: 'https://user:password@example.test/' },
    { ...proof, address: 'ftp://example.test/' },
  ])
    assert.equal(verifiedClaimAddress(invalid, proof.short_id), undefined);
  assert.equal(verifiedClaimAddress(proof, 'other1234'), undefined);
  assert.equal(claimDestination({ ...proof, can_manage: false }), null);
});
test('claim filters are bounded and manual/expired states have distinct recovery labels', () => {
  assert.equal(claimLabel({ ...claimFixture, method: 'MANUAL', expires_at: undefined }), '待审核');
  assert.equal(claimLabel({ ...claimFixture, expires_at: '2020-01-01T00:00:00Z' }), '已过期');
  const { page, query } = claimListQuery(
    new URLSearchParams('page=2&status=PENDING&method=MANUAL&extra=private'),
  );
  assert.equal(page, 2);
  assert.equal(query.toString(), 'page=2&page_size=20&status=PENDING&method=MANUAL');
  assert.equal(
    claimListQuery(new URLSearchParams('page=-1&status=INVALID')).query.toString(),
    'page=1&page_size=20',
  );
  assert.equal(
    claimListQuery(new URLSearchParams(), { management: true }).query.toString(),
    'page=1&page_size=20&status=PENDING&method=MANUAL',
  );
  assert.equal(
    claimListQuery(new URLSearchParams('status=&method='), { management: true }).query.toString(),
    'page=1&page_size=20',
  );
  assert.equal(
    claimListQuery(new URLSearchParams('page=2&status=VERIFIED'), {
      management: true,
    }).query.toString(),
    'page=2&page_size=20&status=VERIFIED&method=MANUAL',
  );
});
test('verification errors preserve typed codes without leaking diagnostic text or retrying mutations', async () => {
  let calls = 0;
  const transport = createClaimsTransport(async () => {
    calls += 1;
    return Response.json(
      { code: 'verification_timeout', detail: 'private upstream diagnostic' },
      { status: 503 },
    );
  });
  await assert.rejects(
    transport.change(claimFixture.id, 'check', new AbortController().signal),
    (error: unknown) => error instanceof ClaimProblem && error.code === 'verification_timeout',
  );
  assert.equal(calls, 1);
  assert.equal(claimMessage('verification_timeout'), '验证超时，请稍后再检查。');
  assert.equal(claimMessage('private diagnostic').includes('private'), false);
});
