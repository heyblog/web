import assert from 'node:assert/strict';
import test from 'node:test';

import { authErrorMessage, authStatusMessage } from '../src/application/auth/auth-page.ts';

test('maps mail protection and validity states to stable user-facing copy', () => {
  assert.equal(authErrorMessage('rate_limited'), '请求过于频繁，请稍后再试。');
  assert.match(authStatusMessage('verification-sent') ?? '', /10 分钟/);
  assert.match(authStatusMessage('reset-sent') ?? '', /30 分钟/);
});
