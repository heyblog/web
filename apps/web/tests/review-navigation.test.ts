import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { stripTypeScriptTypes } from 'node:module';
import { runInNewContext } from 'node:vm';
import test from 'node:test';

import ts from 'typescript';

const source = stripTypeScriptTypes(
  readFileSync(
    new URL(
      '../src/application/management/../site-submission/review-navigation.browser.ts',
      import.meta.url,
    ),
    'utf8',
  ),
).replace('export function', 'function');

function createNavigation() {
  const windowEvents = new Map<string, (event: Event) => void>();
  const documentEvents = new Map<string, (event: Event) => void>();
  const state = { busy: true };
  let confirmations = 0;
  const link = {
    target: '',
    href: 'https://web.example.test/management/users',
    download: false,
    hasAttribute(name: string) {
      return name === 'download' && this.download;
    },
  };
  class Element {
    closest() {
      return link;
    }
  }
  const context = {
    state,
    Element,
    URL,
    location: new URL('https://web.example.test/management/users/editor'),
    window: {
      addEventListener: (name: string, listener: (event: Event) => void) =>
        windowEvents.set(name, listener),
      removeEventListener: (name: string) => windowEvents.delete(name),
      confirm: () => {
        confirmations++;
        return true;
      },
    },
    document: {
      addEventListener: (name: string, listener: (event: Event) => void) =>
        documentEvents.set(name, listener),
      removeEventListener: (name: string) => documentEvents.delete(name),
    },
  };
  runInNewContext(
    `${source}\nglobalThis.cleanup = protectReviewRequest(() => state.busy);`,
    context,
  );
  return {
    state,
    link,
    confirmations: () => confirmations,
    click: (
      modifiers: Readonly<
        Partial<Pick<MouseEvent, 'button' | 'ctrlKey' | 'metaKey' | 'shiftKey' | 'altKey'>>
      > = {},
    ) => {
      const event = Object.assign(new Event('click', { cancelable: true }), {
        button: 0,
        ...modifiers,
      });
      Object.defineProperty(event, 'target', { value: new Element() });
      documentEvents.get('click')?.(event);
      return event;
    },
    unload: () => {
      const event = new Event('beforeunload', { cancelable: true });
      windowEvents.get('beforeunload')?.(event);
      return event;
    },
    cleanup: () => {
      runInNewContext('cleanup()', context);
    },
  };
}

test('review saves block departure until all work finishes', () => {
  const navigation = createNavigation();
  assert.equal(navigation.click().defaultPrevented, true);
  assert.equal(navigation.unload().defaultPrevented, true);
  assert.equal(navigation.confirmations(), 0);
  navigation.state.busy = false;
  assert.equal(navigation.click().defaultPrevented, false);
  assert.equal(navigation.unload().defaultPrevented, false);
  navigation.state.busy = true;
  navigation.cleanup();
  assert.equal(navigation.click().defaultPrevented, false);
  assert.equal(navigation.unload().defaultPrevented, false);
});

test('opening a new tab does not release review request protection', () => {
  for (const modifiers of [
    { ctrlKey: true },
    { metaKey: true },
    { shiftKey: true },
    { button: 1 },
  ]) {
    const navigation = createNavigation();
    assert.equal(navigation.click(modifiers).defaultPrevented, false);
    assert.equal(navigation.unload().defaultPrevented, true);
    navigation.cleanup();
  }
});

function reviewHandler(component: string, name: string): string {
  const text = readFileSync(
    new URL(`../src/components/management/site-submissions/${component}.svelte`, import.meta.url),
    'utf8',
  );
  const script = text.match(/<script lang="ts">([\s\S]*?)<\/script>/u)?.[1];
  assert.ok(script);
  const parsed = ts.createSourceFile(component, script, ts.ScriptTarget.Latest, true);
  const handler = parsed.statements.find(
    (statement) => ts.isFunctionDeclaration(statement) && statement.name?.text === name,
  );
  assert.ok(handler);
  return stripTypeScriptTypes(handler.getText(parsed));
}

for (const action of ['APPROVED', 'REJECTED', 'DISCARD', 'CORRECTION']) {
  test(`successful ${action} releases protection before navigating`, async () => {
    // Given: the actual component handler and the real pending-request guard.
    const navigation = createNavigation();
    navigation.state.busy = false;
    const departures: boolean[] = [];
    const context = {
      get pending() {
        return navigation.state.busy;
      },
      set pending(value: boolean) {
        navigation.state.busy = value;
      },
      pendingAction: action,
      reloadRequired: false,
      error: '',
      availability: { checking: false, check: async () => true },
      addressContext: {},
      canCorrect: true,
      comment: '审核意见',
      confirmDialog: { close() {} },
      detail: {
        id: 'audit',
        action: 'UPDATE',
        current_snapshot: { revision: 1 },
        review_draft_revision: 1,
      },
      form: {},
      accessScope: 'ALL',
      returnPath: '/management/site-submissions',
      requestAuditReview: async () => new Response(null, { status: 204 }),
      requestReviewDraft: async () => new Response(null, { status: 204 }),
      validateSubmissionStep: () => ({ valid: true }),
      buildSubmissionPayload: () => ({ site: {} }),
      URL,
      window: {
        location: {
          origin: 'https://web.example.test',
          assign: () => departures.push(navigation.unload().defaultPrevented),
          reload: () => departures.push(navigation.unload().defaultPrevented),
        },
      },
    };
    const correction = action === 'CORRECTION';
    const name = correction ? 'save' : 'confirmAction';
    const handler = reviewHandler(correction ? 'AuditCorrectionEditor' : 'AuditReviewPanel', name);
    try {
      // When: the API accepts the operation and navigation runs immediately.
      await runInNewContext(`${handler}\n${name}()`, context);
      // Then: no unload prompt is raised at the navigation call itself.
      assert.deepEqual(departures, [false]);
      assert.equal(navigation.state.busy, false);
    } finally {
      navigation.cleanup();
    }
  });
}
