import { marked, type Token, type Tokens } from 'marked';

function inlineText(tokens: readonly Token[]): string {
  return tokens
    .map((token) => {
      if ('tokens' in token && Array.isArray(token.tokens)) return inlineText(token.tokens);
      if (token.type === 'html') return '';
      return 'text' in token && typeof token.text === 'string' ? token.text : '';
    })
    .join('');
}

export function contentDescription(source: string | undefined, fallback: string): string {
  const paragraph = source
    ? marked.lexer(source).find((token): token is Tokens.Paragraph => token.type === 'paragraph')
    : undefined;
  const text = paragraph ? inlineText(paragraph.tokens).replace(/\s+/g, ' ').trim() : '';
  return Array.from(text || fallback)
    .slice(0, 160)
    .join('');
}
