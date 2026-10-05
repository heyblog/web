import assert from 'node:assert/strict';
import test from 'node:test';

import {
  createFullscreenEscapeGuard,
  createGraphFullscreen,
  type FullscreenPort,
  type GraphFullscreenMode,
} from '../src/application/site-graph/site-graph.fullscreen.ts';

function harness(nativeSupported = true) {
  let native = false;
  let enter = async () => {
    native = true;
  };
  let exit = async () => {
    native = false;
  };
  const modes: GraphFullscreenMode[] = [];
  const notices: string[] = [];
  const calls = { enter: 0, exit: 0 };
  const port: FullscreenPort = {
    nativeSupported,
    isNative: () => native,
    enterNative: () => {
      calls.enter++;
      return enter();
    },
    exitNative: () => {
      calls.exit++;
      return exit();
    },
    change: (mode) => modes.push(mode),
    notify: (message) => notices.push(message),
  };
  return {
    controller: createGraphFullscreen(port),
    modes,
    notices,
    calls,
    setNative(value: boolean) {
      native = value;
    },
    setEnter(action: () => Promise<void>) {
      enter = action;
    },
    setExit(action: () => Promise<void>) {
      exit = action;
    },
  };
}

test('page fullscreen toggles synchronously without requesting browser fullscreen', async () => {
  // Given
  const { controller, modes, calls } = harness();
  // When
  const opened = controller.togglePage();
  assert.equal(controller.mode, 'page');
  await opened;
  await controller.togglePage();
  // Then
  assert.deepEqual(modes, ['page', 'inline']);
  assert.deepEqual(calls, { enter: 0, exit: 0 });
});

for (const origin of ['inline', 'page'] as const) {
  test(`native fullscreen returns to its ${origin} origin`, async () => {
    // Given
    const { controller, modes } = harness();
    if (origin === 'page') await controller.togglePage();
    // When
    await controller.toggleNative();
    await controller.toggleNative();
    // Then
    assert.equal(controller.mode, origin);
    assert.deepEqual(modes, origin === 'page' ? ['page', 'native', 'page'] : ['native', 'inline']);
  });
}

test('switching native fullscreen to page changes the subsequent return mode', async () => {
  // Given
  const { controller, setNative } = harness();
  await controller.toggleNative();
  // When
  await controller.togglePage();
  await controller.toggleNative();
  setNative(false);
  controller.nativeChanged();
  // Then
  assert.equal(controller.mode, 'page');
  await controller.close();
  assert.equal(controller.mode, 'inline');
});

test('Escape returns native to page and a second press returns page to inline', async () => {
  // Given
  const { controller } = harness();
  await controller.togglePage();
  await controller.toggleNative();
  // When
  await controller.escape();
  assert.equal(controller.mode, 'page');
  await controller.escape();
  // Then
  assert.equal(controller.mode, 'inline');
});

test('Escape key and native exit events consume one layer in either dispatch order', () => {
  // Given
  const first = createFullscreenEscapeGuard();
  const second = createFullscreenEscapeGuard();
  // When
  first.nativeExited();
  const nativeFirst = first.press();
  const keyFirst = second.press();
  second.nativeExited();
  // Then
  assert.equal(nativeFirst, false);
  assert.equal(keyFirst, true);
  assert.equal(second.press(), false);
  first.release();
  second.release();
  assert.equal(first.press(), true);
  assert.equal(second.press(), true);
});

test('unsupported and rejected native entry preserve page mode and report safe notices', async () => {
  // Given
  const unsupported = harness(false);
  const rejected = harness();
  rejected.setEnter(async () => {
    throw new Error('internal browser diagnostic');
  });
  // When
  for (const { controller } of [unsupported, rejected]) {
    await controller.togglePage();
    await controller.toggleNative();
  }
  // Then
  assert.equal(unsupported.controller.mode, 'page');
  assert.equal(rejected.controller.mode, 'page');
  assert.equal(unsupported.calls.enter, 0);
  assert.equal(rejected.calls.enter, 1);
  assert.equal(unsupported.notices.length, 1);
  assert.equal(rejected.notices.length, 1);
  assert.doesNotMatch(rejected.notices[0] ?? '', /internal|diagnostic/);
});

test('rejected native exit preserves native mode until a successful retry', async () => {
  // Given
  const state = harness();
  await state.controller.toggleNative();
  state.setExit(async () => {
    throw new Error('exit denied');
  });
  // When
  await state.controller.close();
  // Then
  assert.equal(state.controller.mode, 'native');
  assert.equal(state.notices.length, 1);
  state.setExit(async () => {
    state.setNative(false);
  });
  await state.controller.close();
  assert.equal(state.controller.mode, 'inline');
});

test('close during a pending native request exits the late native result', async () => {
  // Given
  const state = harness();
  const entry = Promise.withResolvers<void>();
  state.setEnter(async () => {
    await entry.promise;
    state.setNative(true);
  });
  const entering = state.controller.toggleNative();
  assert.equal(state.calls.enter, 1);
  // When
  const closing = state.controller.close();
  entry.resolve();
  await Promise.all([entering, closing]);
  // Then
  assert.equal(state.controller.mode, 'inline');
  assert.deepEqual(state.calls, { enter: 1, exit: 1 });
  assert.deepEqual(state.modes, []);
});

test('rapid page selection during native entry keeps the latest page intent', async () => {
  // Given
  const state = harness();
  const entry = Promise.withResolvers<void>();
  state.setEnter(async () => {
    await entry.promise;
    state.setNative(true);
  });
  const entering = state.controller.toggleNative();
  // When
  const page = state.controller.togglePage();
  entry.resolve();
  await Promise.all([entering, page]);
  // Then
  assert.equal(state.controller.mode, 'page');
  assert.deepEqual(state.modes, ['page']);
  assert.deepEqual(state.calls, { enter: 1, exit: 1 });
});

test('destroy releases committed mode and cleans a pending native entry', async () => {
  // Given
  const state = harness();
  await state.controller.togglePage();
  const entry = Promise.withResolvers<void>();
  state.setEnter(async () => {
    await entry.promise;
    state.setNative(true);
  });
  const entering = state.controller.toggleNative();
  // When
  state.controller.destroy();
  entry.resolve();
  await entering;
  // Then
  assert.equal(state.controller.mode, 'inline');
  assert.deepEqual(state.modes, ['page', 'inline']);
  assert.deepEqual(state.calls, { enter: 1, exit: 1 });
  await state.controller.togglePage();
  assert.equal(state.controller.mode, 'inline');
});

test('rapid native reentry failure returns to the committed page mode after exit', async () => {
  // Given
  const state = harness();
  await state.controller.togglePage();
  await state.controller.toggleNative();
  const exiting = Promise.withResolvers<void>();
  state.setExit(async () => {
    await exiting.promise;
    state.setNative(false);
  });
  state.setEnter(async () => {
    throw new Error('entry denied');
  });
  const leaving = state.controller.toggleNative();
  // When
  const reentering = state.controller.toggleNative();
  exiting.resolve();
  await Promise.all([leaving, reentering]);
  // Then
  assert.equal(state.controller.mode, 'page');
  assert.equal(state.notices.length, 1);
});
