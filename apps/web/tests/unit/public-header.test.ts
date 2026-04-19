import { describe, expect, it } from 'vitest';

import { resolvePublicHeaderActiveState } from '@/components/public-header/public-header.shared';

describe('public header active state', () => {
  it('resolves primary navigation by path', () => {
    expect(resolvePublicHeaderActiveState({ currentPath: '/site' }).activePrimaryKey).toBe(
      'directory',
    );
    expect(resolvePublicHeaderActiveState({ currentPath: '/site/go' }).activePrimaryKey).toBe(
      'random',
    );
    expect(
      resolvePublicHeaderActiveState({ currentPath: '/site/subscription' }).activePrimaryKey,
    ).toBe('subscription');
    expect(resolvePublicHeaderActiveState({ currentPath: '/site/stats' }).activePrimaryKey).toBe(
      'stats',
    );
  });

  it('resolves project navigation by path', () => {
    expect(resolvePublicHeaderActiveState({ currentPath: '/docs/install' }).activeProjectKey).toBe(
      'docs',
    );
    expect(resolvePublicHeaderActiveState({ currentPath: '/blog/weekly' }).activeProjectKey).toBe(
      'blog',
    );
    expect(resolvePublicHeaderActiveState({ currentPath: '/contribute' }).activeProjectKey).toBe(
      'contribute',
    );
    expect(resolvePublicHeaderActiveState({ currentPath: '/about' }).activeProjectKey).toBe(
      'about',
    );
  });

  it('allows explicit page-level overrides', () => {
    const state = resolvePublicHeaderActiveState({
      currentPath: '/site/example',
      activePrimaryKey: 'directory',
    });

    expect(state.activePrimaryKey).toBe('directory');
    expect(state.activeProjectKey).toBeNull();
  });
});
