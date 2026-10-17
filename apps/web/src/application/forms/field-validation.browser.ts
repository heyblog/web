import { characterLength, utf8Length } from '../../shared/text-length.ts';
import { displayNameIssue, passwordIssue, usernameIssue } from '../auth/auth-fields.ts';
export type Field = HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement;

function label(field: Field): string {
  return (
    field.dataset.fieldLabel ??
    Array.from(field.labels?.[0]?.childNodes ?? [])
      .filter((node) => node.nodeType === Node.TEXT_NODE)
      .map((node) => node.textContent?.trim())
      .find(Boolean) ??
    '此项'
  );
}

export function fieldIssue(field: Field): string {
  const value = field.type === 'password' ? field.value : field.value.trim();
  if (field.required && !value) return `请填写${label(field)}。`;
  if (field.validity.valueMissing) return `请选择${label(field)}。`;
  if (!value) return '';
  if (field.dataset.customIssue) return field.dataset.customIssue;
  switch (field.dataset.validation) {
    case 'username':
      return usernameIssue(value);
    case 'display-name':
      return displayNameIssue(value);
    case 'new-password':
      return passwordIssue(value);
    case 'taxonomy-key':
      if (!/^[a-z0-9]+(-[a-z0-9]+)*\/[a-z0-9]+(-[a-z0-9]+)*$/.test(value))
        return '请填写两段小写字母或数字标识，用 / 分隔，段内可使用连字符。';
      break;
    case 'http-url':
    case 'proof-url': {
      try {
        const url = new URL(value);
        if (
          !['http:', 'https:'].includes(url.protocol) ||
          !url.hostname ||
          (field.dataset.validation === 'proof-url' && (url.username || url.password))
        )
          return field.dataset.validation === 'proof-url' && (url.username || url.password)
            ? '证据链接不能包含登录信息，请使用公开地址。'
            : '请填写有效的 HTTP 或 HTTPS 地址。';
      } catch {
        return '请填写完整的 HTTP 或 HTTPS 地址。';
      }
      break;
    }
  }
  if (field.dataset.match) {
    const other = field.form?.elements.namedItem(field.dataset.match);
    if (other instanceof HTMLInputElement && field.value !== other.value)
      return '两次输入的密码不一致，请重新确认。';
  }
  const maxBytes = Number(field.dataset.maxBytes);
  if (maxBytes && utf8Length(value) > maxBytes)
    return `${label(field)}最多 ${maxBytes} 字节，中文和表情占多个字节。`;
  const maxCharacters = Number(field.dataset.maxCharacters);
  if (maxCharacters && characterLength(value) > maxCharacters)
    return `${label(field)}最多 ${maxCharacters} 个字符。`;
  const validity = field.validity;
  if (validity.typeMismatch)
    return field.type === 'email'
      ? '请输入有效的邮箱地址，例如 name@example.com。'
      : '请输入有效的地址。';
  if (validity.patternMismatch)
    return field.dataset.formatMessage ?? `请检查${label(field)}的格式。`;
  if (validity.badInput) return `请填写有效的${label(field)}。`;
  if (validity.rangeUnderflow && field instanceof HTMLInputElement)
    return `${label(field)}不能小于 ${field.min}。`;
  if (validity.rangeOverflow && field instanceof HTMLInputElement)
    return `${label(field)}不能大于 ${field.max}。`;
  if (validity.stepMismatch) return `${label(field)}需填写符合要求的数值。`;
  if (validity.tooShort && field instanceof HTMLInputElement)
    return `${label(field)}至少需要 ${field.minLength} 个字符。`;
  if (
    validity.tooLong &&
    (field instanceof HTMLInputElement || field instanceof HTMLTextAreaElement)
  )
    return `${label(field)}最多 ${field.maxLength} 个字符。`;
  return '';
}
