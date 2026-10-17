import { characterLength, utf8Length } from '../../shared/text-length.ts';

export function usernameIssue(value: string): string {
  if (!value.trim()) return '请填写用户名。';
  return /^[a-z0-9_]{3,32}$/.test(value.trim()) ? '' : '用户名需为 3–32 位小写字母、数字或下划线。';
}

export function displayNameIssue(value: string): string {
  if (!value.trim()) return '请填写显示名称。';
  return characterLength(value.trim()) <= 128 ? '' : '显示名称最多 128 个字符。';
}

export function passwordIssue(value: string): string {
  if (!value) return '请填写密码。';
  const length = utf8Length(value);
  return length >= 8 && length <= 128 ? '' : '密码需为 8–128 字节，中文和表情占多个字节。';
}

export function authErrorField(code: string | null): string | undefined {
  switch (code) {
    case 'invalid_username':
    case 'username_taken':
      return 'username';
    case 'invalid_email':
    case 'email_taken':
      return 'email';
    case 'invalid_password':
      return 'password';
    case 'password_mismatch':
      return 'confirm_password';
    case 'invalid_current_password':
      return 'current_password';
    case 'invalid_verification_code':
    case 'expired_verification_code':
    case 'verification_attempts_exceeded':
      return 'code';
    default:
      return undefined;
  }
}
