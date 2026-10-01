export function buildRandomSiteParameters(level1: string, level2: string): URLSearchParams {
  const parameters = new URLSearchParams();
  if (level1) parameters.set('level1', level1);
  if (level1 && level2) parameters.set('level2', level2);
  return parameters;
}
