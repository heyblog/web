import type { ScheduleInput } from '../../api/task-center/tasks.types.ts';
import { utf8Length } from '../../shared/text-length.ts';

export const scheduleLimits = {
  nameBytes: 120,
  userAgentBytes: 256,
  timeoutSeconds: 30,
  concurrency: 16,
} as const;
export type ScheduleFieldError = 'name' | 'user_agent' | 'timeout_seconds' | 'concurrency';
export function invalidScheduleField(input: ScheduleInput): ScheduleFieldError | null {
  if (!input.name.trim() || utf8Length(input.name) > scheduleLimits.nameBytes) return 'name';
  const config = input.configuration;
  if (!config.user_agent || utf8Length(config.user_agent) > scheduleLimits.userAgentBytes)
    return 'user_agent';
  if (
    !Number.isInteger(config.timeout_seconds) ||
    config.timeout_seconds < 1 ||
    config.timeout_seconds > scheduleLimits.timeoutSeconds
  )
    return 'timeout_seconds';
  if (
    !Number.isInteger(config.concurrency) ||
    config.concurrency < 1 ||
    config.concurrency > scheduleLimits.concurrency
  )
    return 'concurrency';
  return null;
}
export const scheduleFieldMessages: Readonly<Record<ScheduleFieldError, string>> = {
  name: '名称不能为空，且不能超过 120 字节。',
  user_agent: 'User-Agent 不能为空，且不能超过 256 字节。',
  timeout_seconds: '请求超时须为 1–30 秒的整数。',
  concurrency: '并发数须为 1–16 的整数。',
};
