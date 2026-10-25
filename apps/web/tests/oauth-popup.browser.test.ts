import assert from 'node:assert/strict';
import test from 'node:test';

import { OAuthProblem } from '../src/api/auth/oauth.browser.ts';
import type { OAuthCompletion, OAuthRequest } from '../src/api/auth/oauth.types.ts';
import { createOAuthPopup } from '../src/application/auth/oauth-popup.browser.ts';
import type { OAuthPopupEnvironment } from '../src/application/auth/oauth-popup-environment.browser.ts';
import {
  oauthMessageType,
  oauthReadyMessageType,
} from '../src/application/auth/oauth-popup-protocol.ts';

const input: OAuthRequest = {
  provider: 'blogsclub',
  intent: 'site-claim',
  next: '/dashboard/claims/claim',
  claimId: 'claim',
};
const completed: OAuthCompletion = {
  provider: 'blogsclub',
  state: 'random-state',
  intent: 'site-claim',
  mode: 'popup',
  next: '/dashboard/claims/claim',
  outcome: 'success',
  claim_id: 'claim',
};

function fixture(blocked = false, channelFailure = false, automaticallyReady = true) {
  const navigations: string[] = [];
  const openedURLs: string[] = [];
  const received: OAuthCompletion[] = [];
  const errors: string[] = [];
  const fallbacks: (string | undefined)[] = [];
  let busy = false;
  let disposed = 0;
  const listeners = new Set<Parameters<OAuthPopupEnvironment['listen']>[0]>();
  let broadcast: ((data: unknown) => void) | undefined;
  let poll: (() => void) | undefined;
  const timeouts = new Map<number, () => void>();
  let start: () => Promise<{
    state: string;
    authorizationURL: string;
    requestID: undefined;
  }> = async () => ({
    state: 'random-state',
    authorizationURL: 'https://www.blogsclub.org/Oauth/Login',
    requestID: undefined,
  });
  const popup = {
    closed: false,
    close() {
      this.closed = true;
    },
    focus() {},
    location: {
      replace(url: string) {
        navigations.push(url);
      },
    },
  };
  const environment: OAuthPopupEnvironment = {
    origin: 'https://web.example.test',
    id: () => 'test-attempt',
    open: (url?: string) => {
      openedURLs.push(url ?? 'about:blank');
      popup.closed = false;
      if (automaticallyReady)
        queueMicrotask(() => {
          for (const listener of listeners)
            listener({
              origin: environment.origin,
              source: popup,
              data: { type: oauthReadyMessageType, attempt: 'test-attempt' },
            });
        });
      return blocked ? null : popup;
    },
    listen(listener) {
      listeners.add(listener);
      return () => {
        disposed += 1;
        listeners.delete(listener);
      };
    },
    channel(state, listener) {
      if (channelFailure) throw new DOMException('channel unavailable', 'SecurityError');
      assert.equal(state, 'random-state');
      broadcast = listener;
      return () => {
        disposed += 1;
        broadcast = undefined;
      };
    },
    interval(callback) {
      poll = callback;
      return () => {
        disposed += 1;
        poll = undefined;
      };
    },
    timeout(callback, milliseconds) {
      timeouts.set(milliseconds, callback);
      return () => {
        disposed += 1;
        timeouts.delete(milliseconds);
      };
    },
  };
  const controller = createOAuthPopup(
    {
      fallback() {},
      busy: (value) => {
        busy = value;
      },
      error: (code, fallback) => {
        errors.push(code);
        fallbacks.push(fallback);
      },
      complete: (result) => received.push(result),
    },
    () => environment,
    () => start(),
  );
  return {
    controller,
    popup,
    navigations,
    openedURLs,
    received,
    errors,
    fallbacks,
    get busy() {
      return busy;
    },
    get disposed() {
      return disposed;
    },
    get pendingResources() {
      return listeners.size + timeouts.size + (broadcast ? 1 : 0) + (poll ? 1 : 0);
    },
    setStart(value: typeof start) {
      start = value;
    },
    send(data: unknown, origin = environment.origin, source: unknown = popup) {
      for (const listener of listeners) listener({ data, origin, source });
    },
    broadcast(data: unknown) {
      broadcast?.(data);
    },
    poll() {
      poll?.();
    },
    timeout(milliseconds = 600_000) {
      timeouts.get(milliseconds)?.();
    },
  };
}

test('popup ignores wrong origin, source, provider, state and claim before accepting its own completion', async () => {
  // Given
  const f = fixture();
  await f.controller.launch(input);
  const message = { ...completed, type: oauthMessageType };
  // When
  f.send(message, 'https://attacker.test');
  f.send(message, undefined, {});
  f.send({ ...message, provider: 'github' });
  f.send({ ...message, state: 'other-state' });
  f.send({ ...message, claim_id: 'another-claim' });
  // Then
  assert.deepEqual(f.received, []);
  assert.equal(f.busy, true);
  f.send(message);
  assert.deepEqual(f.received, [completed]);
  assert.equal(f.busy, false);
  assert.equal(f.popup.closed, true);
  assert.equal(f.pendingResources, 0);
});

test('authorization opens a same-origin waiting page instead of an empty document', async () => {
  // Given
  const f = fixture();
  // When
  await f.controller.launch(input);
  // Then
  const opened = new URL(f.openedURLs[0] ?? '', 'https://web.example.test');
  assert.equal(opened.origin, 'https://web.example.test');
  assert.equal(opened.pathname, '/auth/popup');
  f.controller.dispose();
});

test('ready handshake rejects wrong origin, source and attempt before navigation', async () => {
  const f = fixture(false, false, false);
  const launch = f.controller.launch(input);
  await Promise.resolve();
  const ready = { type: oauthReadyMessageType, attempt: 'test-attempt' };
  f.send(ready, 'https://attacker.test');
  f.send(ready, undefined, {});
  f.send({ ...ready, attempt: 'older-attempt' });
  await Promise.resolve();
  assert.deepEqual(f.navigations, []);
  f.send(ready);
  await launch;
  assert.equal(f.navigations.length, 1);
  f.controller.dispose();
  assert.equal(f.pendingResources, 0);
});

test('ready page waits for a delayed start result and duplicate clicks reuse the popup', async () => {
  const f = fixture();
  let finish: (value: {
    state: string;
    authorizationURL: string;
    requestID: undefined;
  }) => void = () => {};
  f.setStart(
    () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
  );
  const launch = f.controller.launch(input);
  await Promise.resolve();
  await f.controller.launch(input);
  assert.equal(f.openedURLs.length, 1);
  assert.deepEqual(f.navigations, []);
  finish({
    state: completed.state,
    authorizationURL: 'https://www.blogsclub.org/Oauth/Login',
    requestID: undefined,
  });
  await launch;
  assert.equal(f.navigations.length, 1);
  f.controller.dispose();
});

test('a waiting-page deadline closes the popup and ignores late readiness', async () => {
  const f = fixture(false, false, false);
  const launch = f.controller.launch(input);
  f.timeout(10_000);
  await launch;
  f.send({ type: oauthReadyMessageType, attempt: 'test-attempt' });
  assert.deepEqual(f.errors, ['oauth_popup_unavailable']);
  assert.deepEqual(f.navigations, []);
  assert.equal(f.popup.closed, true);
  assert.equal(f.pendingResources, 0);
});

test('start rejection and navigation failure offer recovery and clean all resources', async () => {
  for (const failure of ['start', 'navigation']) {
    const f = fixture();
    if (failure === 'start')
      f.setStart(async () => {
        throw new OAuthProblem('bad_gateway');
      });
    else
      f.popup.location.replace = () => {
        throw new DOMException('unavailable', 'SecurityError');
      };
    await f.controller.launch(input);
    assert.deepEqual(f.errors, [failure === 'start' ? 'bad_gateway' : 'oauth_popup_unavailable']);
    assert.ok(f.fallbacks[0]);
    assert.equal(f.busy, false);
    assert.equal(f.popup.closed, true);
    assert.equal(f.pendingResources, 0);
  }
});

test('an unavailable completion channel reports a recoverable error without rejecting launch', async () => {
  // Given
  const f = fixture(false, true);
  // When
  await f.controller.launch(input);
  // Then
  assert.deepEqual(f.errors, ['oauth_popup_unavailable']);
  assert.ok(f.fallbacks[0]);
  assert.equal(f.busy, false);
  assert.equal(f.popup.closed, true);
});

test('detached opener can complete on its scoped channel after closed-window detection', async () => {
  // Given
  const f = fixture();
  await f.controller.launch(input);
  f.popup.closed = true;
  // When
  f.poll();
  f.broadcast({ ...completed, state: 'wrong', type: oauthMessageType });
  f.broadcast({ ...completed, type: oauthMessageType });
  // Then
  assert.deepEqual(f.errors, ['oauth_popup_closed']);
  assert.deepEqual(f.received, [completed]);
  assert.equal(f.pendingResources, 0);
});

test('blocked popup prepares a claim and offers explicit redirect without starting authorization', async () => {
  // Given
  const f = fixture(true);
  let prepared = false;
  // When
  await f.controller.launch(async () => {
    prepared = true;
    return input;
  });
  // Then
  assert.equal(prepared, true);
  assert.deepEqual(f.navigations, []);
  assert.deepEqual(f.errors, ['oauth_popup_blocked']);
  const fallback = new URL(f.fallbacks[0] ?? '', 'https://web.example.test');
  assert.equal(fallback.searchParams.get('mode'), 'redirect');
  assert.equal(fallback.searchParams.get('claim_id'), 'claim');
  assert.equal(f.busy, false);
});

test('timeout cleans every listener and ignores late results', async () => {
  // Given
  const f = fixture();
  await f.controller.launch(input);
  // When
  f.timeout();
  f.broadcast({ ...completed, type: oauthMessageType });
  // Then
  assert.deepEqual(f.errors, ['oauth_timeout']);
  assert.deepEqual(f.received, []);
  assert.equal(f.pendingResources, 0);
});

test('a window-close exception cannot prevent timeout recovery or resource cleanup', async () => {
  const f = fixture();
  await f.controller.launch(input);
  f.popup.close = () => {
    throw new Error('detached window');
  };
  assert.doesNotThrow(() => f.timeout());
  assert.deepEqual(f.errors, ['oauth_timeout']);
  assert.equal(f.busy, false);
  assert.equal(f.pendingResources, 0);
});

test('disposal during claim creation cannot navigate a window after leaving the page', async () => {
  // Given
  const f = fixture();
  let finish: (value: OAuthRequest) => void = () => {};
  const prepared = new Promise<OAuthRequest>((resolve) => {
    finish = resolve;
  });
  const launch = f.controller.launch(() => prepared);
  // When
  f.controller.dispose();
  finish(input);
  await launch;
  // Then
  assert.equal(f.popup.closed, true);
  assert.deepEqual(f.navigations, []);
  assert.deepEqual(f.received, []);
});

test('denied completion preserves a redirect retry and never reports success', async () => {
  // Given
  const f = fixture();
  await f.controller.launch(input);
  // When
  f.send({ ...completed, type: oauthMessageType, outcome: 'error', error_code: 'oauth_denied' });
  // Then
  assert.deepEqual(f.errors, ['oauth_denied']);
  assert.equal(f.fallbacks.length, 1);
  assert.deepEqual(f.received, []);
  assert.equal(f.pendingResources, 0);
});

test('invalid-state errors can finish a claim popup without server-side claim context', async () => {
  const f = fixture();
  await f.controller.launch(input);
  f.send({
    ...completed,
    type: oauthMessageType,
    intent: 'login',
    claim_id: undefined,
    outcome: 'error',
    error_code: 'invalid_oauth_state',
  });
  assert.deepEqual(f.errors, ['invalid_oauth_state']);
  assert.deepEqual(f.received, []);
  assert.equal(f.pendingResources, 0);
});
