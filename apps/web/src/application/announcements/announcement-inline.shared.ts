import { Lexer } from 'marked';

import { isRecord, safeAnnouncementLink } from '../../api/announcements/announcements.types.ts';

export type InlineNode =
  | { readonly kind: 'text' | 'code'; readonly text: string }
  | { readonly kind: 'strong' | 'em' | 'del'; readonly children: readonly InlineNode[] }
  | {
      readonly kind: 'link';
      readonly href: string;
      readonly external: boolean;
      readonly children: readonly InlineNode[];
    };

function project(tokens: readonly unknown[], depth: number): readonly InlineNode[] {
  return tokens.map((token): InlineNode => {
    if (!isRecord(token) || typeof token.raw !== 'string') return { kind: 'text', text: '' };
    if (depth > 20) return { kind: 'text', text: token.raw };
    switch (token.type) {
      case 'strong':
      case 'em':
      case 'del':
        return {
          kind: token.type,
          children: project(Array.isArray(token.tokens) ? token.tokens : [], depth + 1),
        };
      case 'codespan':
        return { kind: 'code', text: typeof token.text === 'string' ? token.text : token.raw };
      case 'link': {
        if (typeof token.href !== 'string') return { kind: 'text', text: token.raw };
        const external = !token.href.startsWith('/');
        if (!safeAnnouncementLink(token.href, external)) return { kind: 'text', text: token.raw };
        return {
          kind: 'link',
          href: token.href,
          external,
          children: project(Array.isArray(token.tokens) ? token.tokens : [], depth + 1),
        };
      }
      default:
        return { kind: 'text', text: token.raw };
    }
  });
}

export function parseAnnouncementInline(source: string): readonly InlineNode[] {
  return project(Lexer.lexInline(source, { gfm: true, breaks: false }), 0);
}
