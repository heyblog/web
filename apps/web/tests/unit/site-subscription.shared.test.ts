import { describe, expect, it } from 'vitest';

import { resolveSubscriptionIntro } from '@/components/site/site-subscription.shared';

describe('site subscription intro', () => {
  it('strips markup and keeps short intros readable', () => {
    expect(resolveSubscriptionIntro('<p>Hello&nbsp;<strong>World</strong></p>')).toBe(
      'Hello World',
    );
  });

  it('truncates long article bodies into short intros', () => {
    const longText = 'A'.repeat(180);

    expect(resolveSubscriptionIntro(longText, 20)).toBe('AAAAAAAAAAAAAAAAAAAA...');
  });
});
