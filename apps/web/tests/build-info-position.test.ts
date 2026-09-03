import assert from 'node:assert/strict';
import test from 'node:test';

import { resolvePopoverPosition } from '../src/shared/build-info-position.shared.ts';

const viewport = { left: 0, top: 0, right: 1280, bottom: 800 } as const;
const panel = { width: 352, height: 280 } as const;

test('places the build information above the trigger when space is available', () => {
  // Given: a footer trigger with ample room above it.
  const trigger = { left: 100, top: 700, right: 300, bottom: 740, width: 200, height: 40 } as const;

  // When: the anchored position is resolved.
  const position = resolvePopoverPosition({ trigger, panel, viewport, padding: 16, gap: 12 });

  // Then: the panel opens above and remains centered on the trigger.
  assert.deepEqual(position, { side: 'above', left: 24, top: 408 });
});

test('places the build information below the trigger when the upper space is insufficient', () => {
  // Given: a trigger near the top of the viewport.
  const trigger = { left: 540, top: 40, right: 740, bottom: 80, width: 200, height: 40 } as const;

  // When: the anchored position is resolved.
  const position = resolvePopoverPosition({ trigger, panel, viewport, padding: 16, gap: 12 });

  // Then: the panel opens below the trigger.
  assert.deepEqual(position, { side: 'below', left: 464, top: 92 });
});

test('clamps the build information to every viewport edge', () => {
  // Given: a narrow viewport and a trigger beyond its lower-right usable area.
  const narrowViewport = { left: 20, top: 30, right: 395, bottom: 700 } as const;
  const widePanel = { width: 352, height: 680 } as const;
  const trigger = { left: 360, top: 660, right: 400, bottom: 700, width: 40, height: 40 } as const;

  // When: neither side can fully contain the panel.
  const position = resolvePopoverPosition({
    trigger,
    panel: widePanel,
    viewport: narrowViewport,
    padding: 16,
    gap: 12,
  });

  // Then: the panel chooses the larger side and stays within the available bounds.
  assert.deepEqual(position, { side: 'above', left: 36, top: 46 });
});
