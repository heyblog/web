import assert from 'node:assert/strict';
import test from 'node:test';

import { newSchedule } from '../src/application/task-center/model.ts';
import { invalidScheduleField } from '../src/application/task-center/schedule-form.ts';

const input = { ...newSchedule(), name: '检测' };
test('form accepts request limits at the API boundary', () => {
  // Given the maximum API-supported timeout/concurrency and a 255-byte UTF-8 UA.
  const value = {
    ...input,
    configuration: {
      ...input.configuration,
      timeout_seconds: 30,
      concurrency: 16,
      user_agent: '中'.repeat(85),
    },
  };
  // When validating a draft before sending it.
  const field = invalidScheduleField(value);
  // Then valid boundary values remain available.
  assert.equal(field, null);
});
test('form rejects values that exceed the API limits', () => {
  for (const [field, value] of [
    ['timeout_seconds', 31],
    ['concurrency', 17],
  ] as const) {
    // Given a value formerly accepted by the HTML control but rejected by Go.
    const draft = { ...input, configuration: { ...input.configuration, [field]: value } };
    // When submitting the draft.
    const error = invalidScheduleField(draft);
    // Then it is caught at the specific field before issuing a request.
    assert.equal(error, field);
  }
});
test('Unicode limits count UTF-8 bytes rather than character length', () => {
  // Given a 258-byte UA whose 86 characters fit the HTML character limit.
  const draft = {
    ...input,
    configuration: { ...input.configuration, user_agent: '中'.repeat(86) },
  };
  // When validating the request configuration.
  const field = invalidScheduleField(draft);
  // Then the draft cannot reach the API with an oversized UA.
  assert.equal(field, 'user_agent');
});
test('Unicode schedule names respect the API byte limit', () => {
  // Given a 123-byte name that fits the 120-character HTML limit.
  // When validating the schedule draft.
  const field = invalidScheduleField({ ...input, name: '中'.repeat(41) });
  // Then the name is rejected before the request is dispatched.
  assert.equal(field, 'name');
});
