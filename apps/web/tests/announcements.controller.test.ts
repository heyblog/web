import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { registerHooks, stripTypeScriptTypes } from 'node:module';
import test from 'node:test';

import { compileModule } from 'svelte/compiler';

import type { ManagedAnnouncement } from '../src/api/announcements/announcements.types.ts';
import { protectFormDraft } from '../src/application/navigation/form-navigation.browser.ts';

const url = new URL(
  '../src/application/announcements/announcement-editor.svelte.ts',
  import.meta.url,
);
const compiled = compileModule(stripTypeScriptTypes(await readFile(url, 'utf8')), {
  filename: url.pathname,
  generate: 'client',
}).js.code;
const hooks = registerHooks({
  load(path, context, next) {
    return path === url.href
      ? { source: compiled, format: 'module', shortCircuit: true }
      : next(path, context);
  },
});
const { createAnnouncementEditor } =
  await import('../src/application/announcements/announcement-editor.svelte.ts');
hooks.deregister();

const announcement: ManagedAnnouncement = {
  id: '0194a290-46e0-7000-8000-000000000001',
  kind: 'MAIN',
  title: '公告',
  bodyMarkdown: null,
  priority: 0,
  actionType: 'NONE',
  actionLabel: null,
  actionPath: null,
  actionExternalUrl: null,
  startsAt: null,
  endsAt: null,
  status: 'DRAFT',
  effectiveStatus: 'DRAFT',
  rowVersion: '1',
  createdAt: '2026-10-01T00:00:00Z',
  updatedAt: '2026-10-01T00:00:00Z',
  publishedAt: null,
  archivedAt: null,
};

for (const action of ['create', 'delete'] as const) {
  for (const success of [true, false]) {
    test(`announcement ${action} ${success ? 'leaves without protection after success' : 'retains draft protection after failure'}`, async (t) => {
      // Given: a dirty editor protected while the API request is in flight.
      let respond: (response: Response) => void = () => {};
      t.mock.method(
        globalThis,
        'fetch',
        () => new Promise<Response>((resolve) => (respond = resolve)),
      );
      const editor = createAnnouncementEditor(action === 'create' ? null : announcement, []);
      editor.draft.title = '修订公告';
      const page = new EventTarget();
      const unload = () => {
        const event = new Event('beforeunload', { cancelable: true });
        page.dispatchEvent(event);
        return event.defaultPrevented;
      };
      const departures: { path: string; protected: boolean }[] = [];
      Object.assign(page, {
        location: {
          assign: (path: string) => departures.push({ path, protected: unload() }),
        },
      });
      const originals = ['window', 'document'].map(
        (name) => [name, Object.getOwnPropertyDescriptor(globalThis, name)] as const,
      );
      Object.defineProperty(globalThis, 'window', { configurable: true, value: page });
      Object.defineProperty(globalThis, 'document', {
        configurable: true,
        value: new EventTarget(),
      });
      const cleanup = protectFormDraft(() => editor);
      try {
        // When: creation/deletion is submitted and then accepted or rejected.
        const pending = action === 'create' ? editor.save() : editor.delete();
        assert.equal(editor.busy, true);
        assert.equal(unload(), true);
        respond(
          success
            ? action === 'create'
              ? Response.json({ announcement })
              : new Response(null, { status: 204 })
            : Response.json({ code: 'validation_failed' }, { status: 422 }),
        );
        await pending;

        // Then: only a successful operation clears protection before navigation.
        assert.equal(editor.busy, false);
        assert.equal(editor.dirty, !success);
        assert.equal(unload(), !success);
        assert.deepEqual(
          departures,
          success
            ? [
                {
                  path:
                    action === 'create'
                      ? `/management/announcements/${announcement.id}`
                      : '/management/announcements',
                  protected: false,
                },
              ]
            : [],
        );
      } finally {
        cleanup();
        for (const [name, descriptor] of originals) {
          if (descriptor) Object.defineProperty(globalThis, name, descriptor);
          else Reflect.deleteProperty(globalThis, name);
        }
      }
    });
  }
}
