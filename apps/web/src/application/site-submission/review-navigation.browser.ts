export function protectReviewRequest(readBusy: () => boolean): () => void {
  const beforeUnload = (event: BeforeUnloadEvent) => {
    if (readBusy()) event.preventDefault();
  };
  const beforeLink = (event: MouseEvent) => {
    if (
      !readBusy() ||
      event.defaultPrevented ||
      event.button !== 0 ||
      event.ctrlKey ||
      event.metaKey ||
      event.shiftKey ||
      event.altKey ||
      !(event.target instanceof Element)
    )
      return;
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
    event.preventDefault();
  };
  window.addEventListener('beforeunload', beforeUnload);
  document.addEventListener('click', beforeLink, true);
  return () => {
    window.removeEventListener('beforeunload', beforeUnload);
    document.removeEventListener('click', beforeLink, true);
  };
}
