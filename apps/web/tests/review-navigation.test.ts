import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { stripTypeScriptTypes } from 'node:module';
import { runInNewContext } from 'node:vm';
import test from 'node:test';

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

test('review generation and saves block departure until all work finishes', () => {
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
