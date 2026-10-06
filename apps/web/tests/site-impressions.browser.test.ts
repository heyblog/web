import assert from 'node:assert/strict';
import { setImmediate } from 'node:timers/promises';
import test from 'node:test';

import { parseImpressions } from '../src/api/site-metrics/site-metrics.types.ts';
import { startSiteImpressions } from '../src/application/site-metrics/site-impressions.browser.ts';

class Card {
  isConnected = true;
  id: string;
  constructor(id: string) {
    this.id = id;
  }
  getAttribute() {
    return this.id;
  }
}
class Page extends EventTarget {
  hidden = false;
  body = {};
  cards: Card[] = [];
  querySelectorAll() {
    return this.cards;
  }
}

test('impressions require continuous foreground visibility, deduplicate and survive lifecycle changes', async (t) => {
  // Given a controlled browser environment and clock.
  t.mock.timers.enable({ apis: ['setTimeout'] });
  const document = new Page();
  const window = new EventTarget();
  let intersections: (
    entries: { target: Card; isIntersecting: boolean; intersectionRatio: number }[],
  ) => void = () => undefined;
  let mutate: () => void = () => undefined;
  const observed = new Set<Card>();
  const deliveries: { eventId: string; shortId: string }[][] = [];
  const beacons: string[] = [];
  let fail = true;
  const replacements = {
    window,
    IntersectionObserver: class {
      constructor(callback: typeof intersections) {
        intersections = callback;
      }
      observe(element: Card) {
        observed.add(element);
      }
      unobserve(element: Card) {
        observed.delete(element);
      }
      disconnect() {
        observed.clear();
      }
    },
    MutationObserver: class {
      constructor(callback: () => void) {
        mutate = callback;
      }
      observe() {}
      disconnect() {}
    },
    navigator: {
      sendBeacon: (_url: string, body: Blob) => {
        void body.text().then((value) => beacons.push(value));
        return true;
      },
    },
    fetch: async (_input: unknown, init?: RequestInit) => {
      const value: unknown = JSON.parse(String(init?.body));
      const events = parseImpressions(value);
      assert.ok(events);
      deliveries.push([...events]);
      return new Response(null, { status: fail ? 503 : 204 });
    },
  };
  const descriptors = new Map<string, PropertyDescriptor | undefined>();
  for (const [name, value] of Object.entries(replacements)) {
    descriptors.set(name, Object.getOwnPropertyDescriptor(globalThis, name));
    Object.defineProperty(globalThis, name, { value, configurable: true });
  }
  const first = new Card('A1b2C3d4E');
  const expanded = new Card(first.id);
  document.cards.push(first, expanded);
  const stop = startSiteImpressions(document as unknown as Document);
  t.after(() => {
    stop();
    for (const [name, descriptor] of descriptors) {
      if (descriptor) Object.defineProperty(globalThis, name, descriptor);
      else Reflect.deleteProperty(globalThis, name);
    }
  });
  const visible = (card: Card, ratio: number) =>
    intersections([{ target: card, isIntersecting: ratio > 0, intersectionRatio: ratio }]);
  const tick = async (ms: number) => {
    t.mock.timers.tick(ms);
    await Promise.resolve();
    await Promise.resolve();
    await Promise.resolve();
  };
  // When visibility falls below the threshold or the page becomes hidden, the second resets.
  visible(first, 0.49);
  await tick(1500);
  assert.equal(deliveries.length, 0);
  visible(first, 0.5);
  await tick(900);
  visible(first, 0.4);
  await tick(500);
  assert.equal(deliveries.length, 0);
  visible(first, 0.5);
  await tick(900);
  document.hidden = true;
  document.dispatchEvent(new Event('visibilitychange'));
  await tick(2000);
  document.hidden = false;
  document.dispatchEvent(new Event('visibilitychange'));
  // Then old intersection ratios cannot qualify a resumed page.
  await tick(2000);
  assert.equal(deliveries.length, 0);
  visible(first, 0.5);
  visible(expanded, 1);
  await tick(999);
  assert.equal(deliveries.length, 0);
  await tick(1);
  await tick(150);
  assert.equal(deliveries.length, 1);
  assert.equal(deliveries[0].length, 1);
  assert.equal(deliveries[0][0].shortId, first.id);
  // Failed delivery retries the identical event rather than generating a new one.
  fail = false;
  await tick(5000);
  assert.deepEqual(deliveries[1], deliveries[0]);
  visible(first, 0);
  visible(first, 1);
  await tick(1500);
  assert.equal(deliveries.length, 2);
  // Changing a selected graph node starts a fresh observation; removed nodes never count.
  expanded.id = 'B1b2C3d4E';
  mutate();
  visible(expanded, 1);
  await tick(500);
  expanded.isConnected = false;
  document.cards = [first];
  mutate();
  await tick(1000);
  assert.equal(deliveries.length, 2);
  const replacement = new Card('C1b2C3d4E');
  document.cards.push(replacement);
  mutate();
  visible(replacement, 1);
  await tick(1000);
  // Pagehide flushes a pending event and BFCache restores observation with the same document ID.
  window.dispatchEvent(new Event('pagehide'));
  await setImmediate();
  assert.equal(beacons.length, 1);
  assert.equal(observed.size, 0);
  window.dispatchEvent(new Event('pageshow'));
  assert.equal(observed.size, 2);
  await tick(150);
  assert.equal(deliveries[2][0].shortId, replacement.id);
  assert.equal(deliveries[2][0].eventId, deliveries[0][0].eventId);
  visible(replacement, 1);
  await tick(2000);
  assert.equal(deliveries.length, 3);
});
