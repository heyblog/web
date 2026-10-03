import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { stripTypeScriptTypes } from 'node:module';
import { runInNewContext } from 'node:vm';
import test from 'node:test';

const source = stripTypeScriptTypes(
  readFileSync(
    new URL('../src/application/management/users-navigation.browser.ts', import.meta.url),
    'utf8',
  ),
).replace('export function', 'function');

function createNavigation() {
  const windowEvents = new Map<string, (event: Event) => void>();
  const documentEvents = new Map<string, (event: Event) => void>();
  const state = { dirty: true, busy: false };
  let confirmations = 0;
  let accepted = true;
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
        return accepted;
      },
    },
    document: {
      addEventListener: (name: string, listener: (event: Event) => void) =>
        documentEvents.set(name, listener),
      removeEventListener: (name: string) => documentEvents.delete(name),
    },
  };
  runInNewContext(`${source}\nglobalThis.cleanup = protectUserDraft(() => state);`, context);
  return {
    state,
    link,
    confirmations: () => confirmations,
    decline: () => {
      accepted = false;
    },
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

test('opening another tab or downloading never disables current draft protection', () => {
  for (const modifiers of [
    { ctrlKey: true },
    { metaKey: true },
    { shiftKey: true },
    { button: 1 },
  ]) {
    const navigation = createNavigation();
    navigation.click(modifiers);
    assert.equal(navigation.confirmations(), 0);
    assert.equal(navigation.unload().defaultPrevented, true);
    navigation.cleanup();
    assert.equal(navigation.unload().defaultPrevented, false);
  }
  for (const attributes of [
    { target: '_blank' },
    { download: true },
    { href: 'https://web.example.test/management/users/editor#role' },
  ]) {
    const navigation = createNavigation();
    Object.assign(navigation.link, attributes);
    navigation.click();
    assert.equal(navigation.confirmations(), 0);
    assert.equal(navigation.unload().defaultPrevented, true);
    navigation.cleanup();
  }
});

test('same-page departure confirms once; cancellation and in-flight saves retain protection', () => {
  const accepted = createNavigation();
  assert.equal(accepted.click().defaultPrevented, false);
  assert.equal(accepted.confirmations(), 1);
  assert.equal(accepted.unload().defaultPrevented, false);
  accepted.cleanup();
  const cancelled = createNavigation();
  cancelled.decline();
  assert.equal(cancelled.click().defaultPrevented, true);
  assert.equal(cancelled.unload().defaultPrevented, true);
  cancelled.cleanup();
  const saving = createNavigation();
  saving.state.busy = true;
  assert.equal(saving.click().defaultPrevented, true);
  assert.equal(saving.confirmations(), 0);
  assert.equal(saving.unload().defaultPrevented, true);
  saving.cleanup();
});
