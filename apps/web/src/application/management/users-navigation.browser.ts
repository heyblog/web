export function protectUserDraft(
  readState: () => Readonly<{ dirty: boolean; busy: boolean }>,
): () => void {
  let leaving = false;
  const beforeUnload = (event: BeforeUnloadEvent) => {
    const state = readState();
    if (!leaving && (state.dirty || state.busy)) event.preventDefault();
  };
  const beforeLink = (event: MouseEvent) => {
    if (
      event.defaultPrevented ||
      event.button !== 0 ||
      event.ctrlKey ||
      event.metaKey ||
      event.shiftKey ||
      event.altKey
    )
      return;
    if (!(event.target instanceof Element)) return;
    const link = event.target.closest<HTMLAnchorElement>('a[href]');
    if (!link || link.hasAttribute('download') || (link.target && link.target !== '_self')) return;
    const destination = new URL(link.href);
    if (!['http:', 'https:'].includes(destination.protocol)) return;
    if (
      destination.hash &&
      destination.origin === location.origin &&
      destination.pathname === location.pathname &&
      destination.search === location.search
    )
      return;
    const state = readState();
    if (state.busy || (state.dirty && !window.confirm('离开此页？尚未保存的修改将丢失。')))
      event.preventDefault();
    else leaving = true;
  };
  window.addEventListener('beforeunload', beforeUnload);
  document.addEventListener('click', beforeLink, true);
  return () => {
    window.removeEventListener('beforeunload', beforeUnload);
    document.removeEventListener('click', beforeLink, true);
  };
}
