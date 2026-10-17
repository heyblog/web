import assert from 'node:assert/strict';
import test from 'node:test';

import {
  authErrorField,
  displayNameIssue,
  passwordIssue,
  usernameIssue,
} from '../src/application/auth/auth-fields.ts';
import { characterLength, utf8Length } from '../src/shared/text-length.ts';

test('password validation counts UTF-8 bytes without trimming the password', () => {
  // Given: passwords on both sides of the API's byte boundaries.
  const cases = [
    ['a'.repeat(7), false],
    ['a'.repeat(8), true],
    ['密'.repeat(2), false],
    ['😀'.repeat(2), true],
    ['😀'.repeat(32), true],
    ['密'.repeat(43), false],
    ['a'.repeat(129), false],
    ['       a', true],
  ] as const;
  for (const [value, valid] of cases) {
    // When: a new password is validated.
    const issue = passwordIssue(value);
    // Then: acceptance matches the existing API's 8–128 byte range.
    assert.equal(issue === '', valid);
  }
});

test('display names count Unicode characters after trimming rather than UTF-16 units', () => {
  // Given: whitespace and names containing supplementary Unicode characters.
  const cases = [
    ['  ', false],
    ['😀'.repeat(128), true],
    ['😀'.repeat(129), false],
    [` ${'名'.repeat(128)} `, true],
  ] as const;
  for (const [value, valid] of cases) {
    // When: the display name is validated.
    const issue = displayNameIssue(value);
    // Then: the result agrees with the API's trimmed character limit.
    assert.equal(issue === '', valid);
  }
});

test('username validation accepts only the advertised lowercase username format', () => {
  // Given: empty, short, uppercase, and valid usernames.
  const cases = [
    ['', false],
    ['ab', false],
    ['reader_', true],
    ['Reader', false],
    ['x'.repeat(33), false],
  ] as const;
  for (const [value, valid] of cases) {
    // When: a username is validated.
    const issue = usernameIssue(value);
    // Then: only usernames matching the stated format pass.
    assert.equal(issue === '', valid);
  }
});

test('field mapping keeps credential failures global and identifies recoverable field errors', () => {
  // Given: stable auth problem codes.
  const codes = [
    'username_taken',
    'invalid_current_password',
    'password_mismatch',
    'invalid_credentials',
    'unknown',
  ];
  // When: their field destinations are resolved.
  const fields = codes.map(authErrorField);
  // Then: joint credential failures and unknown errors never imply a specific account field.
  assert.deepEqual(fields, [
    'username',
    'current_password',
    'confirm_password',
    undefined,
    undefined,
  ]);
});

test('byte and character measurements remain distinct for non-ASCII text', () => {
  // Given: text containing ASCII, Chinese, and a supplementary code point.
  const value = 'a中😀';
  // When: each API length unit is measured.
  const lengths = [utf8Length(value), characterLength(value)];
  // Then: each unit retains its defined semantics.
  assert.deepEqual(lengths, [8, 3]);
});
