export async function fadeDialogOut(dialog: HTMLDialogElement): Promise<void> {
  const duration =
    Number.parseFloat(getComputedStyle(dialog).getPropertyValue('--motion-fast')) || 150;
  try {
    await dialog.animate([{ opacity: 1 }, { opacity: 0 }], { duration, easing: 'ease-out' })
      .finished;
  } catch (error) {
    if (!(error instanceof DOMException && error.name === 'AbortError')) throw error;
  }
}
