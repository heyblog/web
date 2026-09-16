let draftSequence = 0;

export function nextDraftID(prefix: string): string {
  draftSequence += 1;
  return prefix + '-' + String(draftSequence);
}
