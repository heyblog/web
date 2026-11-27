import assert from 'node:assert/strict';
import test from 'node:test';

import { ProtocolError } from '../src/api/task-center/parse.ts';
import { parseProof } from '../src/api/task-center/signing.types.ts';
import { taskAPI } from '../src/api/task-center/task-center.browser.ts';
import { forwardTask } from '../src/api/task-center/task-center.proxy.server.ts';
import { parseTarget } from '../src/api/task-center/tasks.types.ts';
import { managementNavigation } from '../src/application/management/navigation.ts';
import { newSchedule } from '../src/application/task-center/model.ts';

const id = '11111111-1111-4111-8111-111111111111';
test('schedule edits serialize only writable fields while preserving revision', async (t) => {
  const input = { ...newSchedule(), name: 'Check', revision: 3 };
  const schedule = { ...input, id, next_at: null };
  let calls = 0;
  t.mock.method(globalThis, 'fetch', async (url: string, init: RequestInit) => {
    calls++;
    assert.equal(url, `/api/web/management/tasks/schedules/${id}`);
    assert.equal(init.method, 'PUT');
    assert.deepEqual(JSON.parse(String(init.body)), input);
    return Response.json(schedule);
  });
  assert.deepEqual(await taskAPI.save(schedule, id), schedule);
  assert.equal(calls, 1);
});
function mutation(body = '{}', origin = 'https://web.example.test') {
  return new Request('https://web.example.test/api/web/management/tasks/schedules', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Cookie: 'test=session',
      Origin: origin,
      'Sec-Fetch-Site': 'same-origin',
    },
    body,
  });
}
test('proxy rejects unknown paths and wrong methods before contacting API', async () => {
  // Given a dispatcher whose network call must remain unreachable.
  const forward = async () => {
    assert.fail('unexpected upstream dispatch');
  };
  // When a path escapes the explicit task operation list.
  const responses = await Promise.all([
    forwardTask(mutation(), 'tasks', '../request-signing/keys', forward),
    forwardTask(mutation(), 'tasks', 'overview', forward),
  ]);
  // Then neither operation is forwarded.
  assert.deepEqual(
    responses.map((r) => r.status),
    [404, 405],
  );
});
test('proxy rejects cross-origin writes and oversized streaming JSON', async () => {
  // Given valid task routes with hostile request metadata or payloads.
  const forward = async () => {
    assert.fail('unexpected upstream dispatch');
  };
  // When requests cross the bounded same-origin interface.
  const responses = await Promise.all([
    forwardTask(mutation('{}', 'https://other.test'), 'tasks', 'schedules', forward),
    forwardTask(
      mutation(JSON.stringify({ value: 'x'.repeat(65536) })),
      'tasks',
      'schedules',
      forward,
    ),
  ]);
  // Then upstream is untouched.
  assert.deepEqual(
    responses.map((r) => r.status),
    [403, 413],
  );
});
test('proxy preserves the exact accepted method and refreshed cookies', async () => {
  // Given an authenticated manual-run request.
  let calls = 0;
  // When forwarding the explicit allowlisted operation.
  const response = await forwardTask(
    mutation(),
    'tasks',
    `schedules/${id}/run`,
    async (request, path, init) => {
      calls++;
      assert.equal(path, `/api/web/management/tasks/schedules/${id}/run`);
      assert.equal(init?.method, 'POST');
      assert.deepEqual(init?.body, {});
      assert.equal(request.headers.get('Cookie'), 'test=session');
      return Response.json({ id }, { headers: { 'Set-Cookie': 'session=renewed; HttpOnly' } });
    },
  );
  // Then mutation is dispatched only once and session renewal survives.
  assert.equal(calls, 1);
  assert.equal(response.status, 200);
  assert.equal(response.headers.get('Cache-Control'), 'private, no-store');
  assert.match(response.headers.get('Set-Cookie') ?? '', /session=renewed/u);
});
test('public proof inspection does not require a management cookie', async () => {
  // Given a same-origin public proof request without authentication.
  const request = mutation(JSON.stringify({ token: '' }));
  request.headers.delete('Cookie');
  // When inspecting missing proof.
  const response = await forwardTask(
    request,
    'public-signing',
    'verify',
    async (_request, path) => {
      assert.equal(path, '/api/web/public/request-signing/verify');
      return Response.json({ status: 'missing' });
    },
  );
  // Then the public route remains accessible.
  assert.deepEqual(parseProof(await response.json()), {
    status: 'missing',
    binding: 'unbound',
    claims: null,
  });
});
test('target parsing excludes leases and raw diagnostics', () => {
  // Given a target DTO containing server-only properties.
  const value = {
    id,
    site_id: id,
    address: 'https://example.test',
    kind: 'monitor',
    state: 'pending',
    result: null,
    sample_started_at: null,
    reason: '',
    usable: false,
    recoveries: 0,
    round_id: id,
    lease_token: 'secret',
    diagnostic: 'private',
  };
  // When exposing the typed browser contract.
  const target = parseTarget(value);
  // Then only permitted display properties survive.
  assert.equal('lease_token' in target, false);
  assert.equal('diagnostic' in target, false);
});
test('task queries preserve bounded filters and mutation idempotency without exposing internal routes', async () => {
  // Given a page read and an idempotent manual creation.
  const read = new Request(
    'https://web.example.test/api/web/management/tasks/runs?page=2&page_size=20&state=paused',
    {
      headers: {
        Cookie: 'session=test',
        Accept: 'application/json',
        Referer: 'https://web.example.test/management/tasks',
      },
    },
  );
  const write = mutation();
  write.headers.set('Idempotency-Key', id);
  const seen: string[] = [];
  const forward = async (
    _request: Request,
    path: `/${string}`,
    init?: { idempotencyKey?: string },
  ) => {
    seen.push(path);
    if (path.endsWith('/runs')) assert.equal(init?.idempotencyKey, id);
    return Response.json({});
  };
  // When valid operations cross the narrow proxy.
  assert.equal((await forwardTask(read, 'tasks', 'runs', forward)).status, 200);
  assert.equal((await forwardTask(write, 'tasks', 'runs', forward)).status, 200);
  const oversized = new Request(read.url.replace('page_size=20', 'page_size=101'), read);
  assert.equal((await forwardTask(oversized, 'tasks', 'runs', forward)).status, 400);
  // Then only the accepted page and single creation reach the API.
  assert.deepEqual(seen, [
    '/api/web/management/tasks/runs?page=2&page_size=20&state=paused',
    '/api/web/management/tasks/runs',
  ]);
});
test('proof parsing rejects malformed success', () => {
  // Given a success-shaped body without trustworthy claims.
  // When parsing its unknown status.
  const parse = () => parseProof({ status: 'trusted', claims: {} });
  // Then it cannot be shown as valid.
  assert.throws(parse, ProtocolError);
});
test('navigation separates task management from system signing authority', () => {
  // Given an admin with task management permission.
  // When building its management navigation.
  const paths = managementNavigation({ role: 'ADMIN', permissions: ['task.manage'] }).map(
    (item) => item.href,
  );
  // Then only the ordinary task surface is available.
  assert.equal(paths.includes('/management/tasks'), true);
  assert.equal(paths.includes('/management/request-signing'), false);
});
