import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { registerHooks, stripTypeScriptTypes } from 'node:module';
import test from 'node:test';

import { compileModule } from 'svelte/compiler';

import { newSchedule } from '../src/application/task-center/model.ts';

const modules = new Map<string, string>();
for (const name of ['tasks.controller', 'run-details']) {
  const url = new URL(`../src/application/task-center/${name}.svelte.ts`, import.meta.url);
  modules.set(
    url.href,
    compileModule(stripTypeScriptTypes(await readFile(url, 'utf8')), {
      filename: url.pathname,
      generate: 'client',
    }).js.code,
  );
}
const hooks = registerHooks({
  load(path, context, next) {
    const source = modules.get(path);
    return source !== undefined
      ? { source, format: 'module', shortCircuit: true }
      : next(path, context);
  },
});

const { TasksController } =
  await import('../src/application/task-center/tasks.controller.svelte.ts');
hooks.deregister();

for (const outcome of ['loaded', 'unavailable', 'disposed'] as const) {
  test(`a user save waits for an in-flight poll: ${outcome}`, async (t) => {
    // Given a slow poll on an active view.
    const document = Object.assign(new EventTarget(), { hidden: false });
    const original = Object.getOwnPropertyDescriptor(globalThis, 'document');
    Object.defineProperty(globalThis, 'document', { value: document, configurable: true });
    const pending = Promise.withResolvers<Response>();
    const input = { ...newSchedule(), name: 'new schedule' };
    const saved = {
      ...input,
      id: '11111111-1111-4111-8111-111111111111',
      revision: 1,
      next_at: null,
    };
    let writes = 0;
    let scheduleReads = 0;
    t.mock.method(globalThis, 'fetch', async (url: string, init?: RequestInit) => {
      if (init?.method === 'POST') {
        writes++;
        return Response.json(saved);
      }
      const path = new URL(url, 'https://example.test').pathname;
      if (path.endsWith('/schedules')) {
        if (scheduleReads++ === 0) return pending.promise;
        return Response.json([saved]);
      }
      if (path.endsWith('/runs'))
        return Response.json({ items: [], page: 1, page_size: 20, total: 0 });
      if (path.endsWith('/overview'))
        return Response.json({ states: {}, recent: [], workers: [], backoff: [] });
      return Response.json([]);
    });
    const controller = new TasksController();
    controller.workers = [
      {
        instance_id: 'worker',
        protocol: 2,
        version: 'test',
        registered_at: '',
        heartbeat_at: '',
        draining: false,
        online: true,
        active: 0,
      },
    ];
    const stop = controller.startPolling();
    t.after(() => {
      stop();
      if (original) Object.defineProperty(globalThis, 'document', original);
      else Reflect.deleteProperty(globalThis, 'document');
    });
    document.dispatchEvent(new Event('visibilitychange'));
    assert.equal(scheduleReads, 1);
    // When a save arrives before the poll settles.
    const saving = controller.save(input);
    assert.equal(controller.busy, true);
    assert.equal(writes, 0);
    if (outcome === 'disposed') stop();
    pending.resolve(
      outcome === 'unavailable'
        ? Response.json({ code: 'service_unavailable' }, { status: 503 })
        : Response.json([]),
    );
    // Then the action runs once after the read, unless its page was disposed.
    assert.equal(await saving, outcome !== 'disposed');
    assert.equal(writes, outcome === 'disposed' ? 0 : 1);
    assert.equal(controller.busy, false);
    if (outcome !== 'disposed') {
      assert.equal(controller.schedules[0]?.id, saved.id);
      assert.equal(controller.refreshError, '');
    }
  });
}
test('successful creation remains accepted when the following refresh fails', async (t) => {
  // Given a POST that creates a schedule and a subsequent failed list refresh.
  const input = { ...newSchedule(), name: 'hourly' };
  const saved = {
    ...input,
    id: '11111111-1111-4111-8111-111111111111',
    revision: 1,
    next_at: null,
  };
  let writes = 0;
  t.mock.method(globalThis, 'fetch', async (_url: string, init?: RequestInit) => {
    if (init?.method === 'POST') {
      writes++;
      return Response.json(saved);
    }
    return Response.json({ code: 'service_unavailable' }, { status: 503 });
  });
  const controller = new TasksController();
  // When creation succeeds but its refresh does not.
  const accepted = await controller.save(input);
  // Then the editor may close, the returned identity is retained, and no retry is sent.
  assert.equal(accepted, true);
  assert.equal(controller.schedules[0]?.id, saved.id);
  assert.equal(controller.schedules[0]?.revision, 1);
  assert.equal(writes, 1);
  assert.equal(controller.error, '');
  assert.notEqual(controller.refreshError, '');
});

test('a rejected save preserves its draft and existing revision', async (t) => {
  // Given a conflicting edit rejected by the authoritative API.
  const saved = {
    ...newSchedule(),
    name: 'hourly',
    id: '11111111-1111-4111-8111-111111111111',
    revision: 3,
    next_at: null,
  };
  t.mock.method(globalThis, 'fetch', async () =>
    Response.json({ code: 'conflict' }, { status: 409 }),
  );
  const controller = new TasksController();
  controller.schedules = [saved];
  // When saving against a stale revision.
  const accepted = await controller.save({ ...saved, name: 'changed' }, saved.id);
  // Then the editor remains open and the displayed schedule is not replaced by the draft.
  assert.equal(accepted, false);
  assert.equal(controller.schedules[0]?.name, 'hourly');
  assert.equal(controller.schedules[0]?.revision, 3);
});
test('accepted manual creation is retained when subsequent reads fail', async (t) => {
  // Given an accepted task whose post-creation refresh is unavailable.
  let writes = 0;
  t.mock.method(globalThis, 'fetch', async (_url: string, init?: RequestInit) => {
    if (init?.method === 'POST') {
      writes++;
      return Response.json({ id: '11111111-1111-4111-8111-111111111111' }, { status: 202 });
    }
    return Response.json({ code: 'service_unavailable' }, { status: 503 });
  });
  const controller = new TasksController();
  // When creating once.
  const accepted = await controller.create({
    kind: 'monitor',
    configuration: newSchedule().configuration,
  });
  // Then a stale list does not turn the accepted operation into a failed creation.
  assert.equal(accepted, true);
  assert.equal(writes, 1);
  assert.equal(controller.error, '');
  assert.notEqual(controller.refreshError, '');
});
