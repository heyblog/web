import assert from 'node:assert/strict';
import test from 'node:test';

import type { OAuthCompletion, OAuthRequest } from '../src/api/auth/oauth.types.ts';
import {
  createOAuthPopup,
  oauthMessageType,
  type OAuthPopupEnvironment,
} from '../src/application/auth/oauth-popup.browser.ts';

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

function fixture(blocked = false) {
  const navigations: string[] = [];
  const received: OAuthCompletion[] = [];
  const errors: string[] = [];
  const fallbacks: (string | undefined)[] = [];
  let busy = false;
  let disposed = 0;
  let message: Parameters<OAuthPopupEnvironment['listen']>[0] | undefined;
  let broadcast: ((data: unknown) => void) | undefined;
  let poll: (() => void) | undefined;
  let timeout: (() => void) | undefined;
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
    open: () => (blocked ? null : popup),
    listen(listener) {
      message = listener;
      return () => {
        disposed += 1;
        message = undefined;
      };
    },
    channel(state, listener) {
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
    timeout(callback) {
      timeout = callback;
      return () => {
        disposed += 1;
        timeout = undefined;
      };
    },
  };
  const controller = createOAuthPopup(
    {
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
    async () => ({
      state: 'random-state',
      authorizationURL: 'https://www.blogsclub.org/Oauth/Login',
    }),
  );
  return {
    controller,
    popup,
    navigations,
    received,
    errors,
    fallbacks,
    get busy() {
      return busy;
    },
    get disposed() {
      return disposed;
    },
    send(data: unknown, origin = environment.origin, source: unknown = popup) {
      message?.({ data, origin, source });
    },
    broadcast(data: unknown) {
      broadcast?.(data);
    },
    poll() {
      poll?.();
    },
    timeout() {
      timeout?.();
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
  assert.equal(f.disposed, 4);
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
  assert.equal(f.disposed, 4);
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
  assert.equal(f.disposed, 4);
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
  assert.equal(f.disposed, 4);
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
  assert.equal(f.disposed, 4);
});
