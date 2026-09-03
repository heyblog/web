export type PopoverSide = 'above' | 'below';

type PopoverPosition = {
  readonly side: PopoverSide;
  readonly left: number;
  readonly top: number;
};

type ResolvePopoverPositionOptions = {
  readonly trigger: {
    readonly left: number;
    readonly top: number;
    readonly right: number;
    readonly bottom: number;
    readonly width: number;
    readonly height: number;
  };
  readonly panel: {
    readonly width: number;
    readonly height: number;
  };
  readonly viewport: {
    readonly left: number;
    readonly top: number;
    readonly right: number;
    readonly bottom: number;
  };
  readonly padding: number;
  readonly gap: number;
};

export function resolvePopoverPosition(_options: ResolvePopoverPositionOptions): PopoverPosition {
  const { trigger, panel, viewport, padding, gap } = _options;
  const minimumLeft = viewport.left + padding;
  const maximumLeft = Math.max(minimumLeft, viewport.right - padding - panel.width);
  const preferredLeft = trigger.left + (trigger.width - panel.width) / 2;
  const spaceAbove = trigger.top - viewport.top - padding - gap;
  const spaceBelow = viewport.bottom - trigger.bottom - padding - gap;
  const canFitAbove = spaceAbove >= panel.height;
  const canFitBelow = spaceBelow >= panel.height;
  const placeAbove = canFitAbove || (!canFitBelow && spaceAbove >= spaceBelow);
  const preferredTop = placeAbove ? trigger.top - panel.height - gap : trigger.bottom + gap;
  const minimumTop = viewport.top + padding;
  const maximumTop = Math.max(minimumTop, viewport.bottom - padding - panel.height);
  const clamp = (value: number, minimum: number, maximum: number) =>
    Math.min(Math.max(value, minimum), maximum);

  return {
    side: placeAbove ? 'above' : 'below',
    left: Math.round(clamp(preferredLeft, minimumLeft, maximumLeft)),
    top: Math.round(clamp(preferredTop, minimumTop, maximumTop)),
  };
}
