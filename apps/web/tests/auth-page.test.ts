import assert from 'node:assert/strict';
import test from 'node:test';

import { authErrorMessage, authStatusMessage } from '../src/application/auth/auth-page.ts';

test('maps mail protection and validity states to stable user-facing copy', () => {
  assert.equal(authErrorMessage('rate_limited'), '请求过于频繁，请稍后再试。');
  assert.match(authStatusMessage('verification-sent') ?? '', /10 分钟/);
  assert.match(authStatusMessage('reset-sent') ?? '', /30 分钟/);
});

test('password failures identify invalid input and expired sessions', () => {
  // Given / When: password setup or change returns a stable failure code.
  // Then: the page explains the corresponding recovery without raw diagnostics.
  assert.equal(authErrorMessage('invalid_current_password'), '当前密码不正确，请重新输入。');
  assert.equal(authErrorMessage('validation_failed'), '提交内容校验失败，请刷新页面后重试。');
  for (const code of ['unauthenticated', 'session_expired', 'invalid_token']) {
    assert.equal(authErrorMessage(code), '登录已过期，请重新登录后重试。');
  }
});
