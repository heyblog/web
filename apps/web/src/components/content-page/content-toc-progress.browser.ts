// These mutable DOM contracts keep the progress presentation testable without a DOM shim.
interface AttributeWriter {
  setAttribute(name: string, value: string): void;
}

interface ProgressValueStyle {
  scale: string;
}

export const resolveContentScrollProgress = (
  scrollY: number,
  scrollHeight: number,
  viewportHeight: number,
): number => {
  const maxScroll = Math.max(scrollHeight - viewportHeight, 0);

  if (maxScroll === 0) {
    return 0;
  }

  return Math.min(1, Math.max(0, scrollY / maxScroll));
};

export const updateContentProgress = (
  progressBar: AttributeWriter,
  valueStyle: ProgressValueStyle | null,
  progress: number,
): void => {
  progressBar.setAttribute('aria-valuenow', String(Math.round(progress * 100)));

  if (valueStyle) {
    valueStyle.scale = `${progress} 1`;
  }
};
