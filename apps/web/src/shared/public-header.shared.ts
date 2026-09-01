interface BrandVisibilityInput {
  home: boolean;
  heroBottom: number | null;
  threshold: number;
}

export function resolveBrandVisibility({
  home,
  heroBottom,
  threshold,
}: BrandVisibilityInput): boolean {
  if (!home) {
    return true;
  }

  return heroBottom !== null && heroBottom <= threshold;
}
