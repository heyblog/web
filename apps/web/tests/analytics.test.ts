import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { runInNewContext } from 'node:vm';
import test from 'node:test';

type AnalyticsWindow = EventTarget & {
  dataLayer?: ArrayLike<unknown>[];
  gtag?: (...args: unknown[]) => void;
  setTimeout: (callback: () => void, delay: number) => number;
};

function createBrowser(readyState: DocumentReadyState) {
  const scripts: { src: string; async: boolean; dataset: Record<string, string> }[] = [];
  const tasks: (() => void)[] = [];
  const window: AnalyticsWindow = Object.assign(new EventTarget(), {
    setTimeout(callback: () => void, delay: number) {
      assert.equal(delay, 0);
      tasks.push(callback);
      return tasks.length;
    },
  });
  const document = {
    readyState,
    createElement(tag: string) {
      assert.equal(tag, 'script');
      return { src: '', async: false, dataset: {} };
    },
    head: {
      append(script: (typeof scripts)[number]) {
        scripts.push(structuredClone(script));
        // Configuration must exist before either remote script can execute.
        assert.equal(typeof window.gtag, 'function');
        assert.deepEqual(Array.from(window.dataLayer?.at(-1) ?? []), ['config', 'G-PH5EGCPHXH']);
      },
    },
  };

  return {
    window,
    scripts,
    tasks,
    execute() {
      runInNewContext(readFileSync(new URL('../public/analytics.js', import.meta.url), 'utf8'), {
        window,
        document,
        Date,
      });
    },
    load() {
      document.readyState = 'complete';
      window.dispatchEvent(new Event('load'));
    },
    flush() {
      for (const task of tasks.splice(0)) task();
    },
  };
}

for (const readyState of ['loading', 'interactive'] as const) {
  test(`analytics waits until after load when the document is ${readyState}`, () => {
    // Given: a document whose load event has not fired.
    const browser = createBrowser(readyState);
    browser.execute();
    assert.equal(browser.tasks.length, 0);
    assert.equal(browser.scripts.length, 0);
    assert.equal(browser.window.gtag, undefined);

    // When: load fires, then its deferred task runs.
    browser.load();
    assert.equal(browser.scripts.length, 0);
    assert.equal(browser.tasks.length, 1);
    browser.flush();

    // Then: both providers start asynchronously with their configuration intact.
    assert.deepEqual(browser.scripts, [
      {
        src: 'https://static.cloudflareinsights.com/beacon.min.js',
        async: true,
        dataset: { cfBeacon: '{"token":"d76b65b3f55b4b8f96a0ac0ddcd5493e","spa":false}' },
      },
      {
        src: 'https://www.googletagmanager.com/gtag/js?id=G-PH5EGCPHXH',
        async: true,
        dataset: {},
      },
    ]);
    assert.equal(browser.window.dataLayer?.length, 2);
    const initialization = Array.from(browser.window.dataLayer?.[0] ?? []);
    assert.equal(initialization[0], 'js');
    assert.ok(initialization[1] instanceof Date);
  });
}

test('analytics initializes after load has already completed and preserves queued events', () => {
  // Given: an already loaded document with an existing data layer.
  const browser = createBrowser('complete');
  const queuedEvent = ['event', 'existing'];
  browser.window.dataLayer = [queuedEvent];

  // When: the loader executes after load.
  browser.execute();
  assert.equal(browser.scripts.length, 0);
  assert.equal(browser.tasks.length, 1);
  browser.flush();

  // Then: both providers initialize and the prior queue remains intact.
  assert.equal(browser.scripts.length, 2);
  assert.equal(browser.window.dataLayer.length, 3);
  assert.equal(browser.window.dataLayer[0], queuedEvent);
});

test('repeated load events do not initialize analytics twice', () => {
  // Given: a loader registered before the page finishes loading.
  const browser = createBrowser('interactive');
  browser.execute();

  // When: the load event is dispatched twice.
  browser.load();
  browser.load();
  browser.flush();

  // Then: each provider and the GA configuration appear exactly once.
  assert.equal(browser.scripts.length, 2);
  assert.equal(browser.window.dataLayer?.length, 2);
});
