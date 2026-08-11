interface AnimatedDetailsOptions {
  panel: HTMLElement;
  onExpandedChange?: (expanded: boolean) => void;
}

interface CloseOptions {
  immediate?: boolean;
  restoreFocus?: boolean;
}

export interface AnimatedDetailsController {
  close: (options?: CloseOptions) => void;
  open: () => void;
  toggle: () => void;
}

const controllers = new WeakMap<HTMLDetailsElement, AnimatedDetailsController>();

const parseTime = (value: string) => {
  const time = Number.parseFloat(value);

  return value.trim().endsWith('ms') ? time : time * 1000;
};

const getTransitionTime = (element: HTMLElement) => {
  const style = getComputedStyle(element);
  const durations = style.transitionDuration.split(',').map(parseTime);
  const delays = style.transitionDelay.split(',').map(parseTime);

  return Math.max(
    0,
    ...durations.map((duration, index) => duration + (delays[index % delays.length] ?? 0)),
  );
};

export const setupAnimatedDetails = (
  details: HTMLDetailsElement,
  { panel, onExpandedChange }: AnimatedDetailsOptions,
): AnimatedDetailsController => {
  const existingController = controllers.get(details);

  if (existingController) {
    return existingController;
  }

  const summary = details.querySelector<HTMLElement>(':scope > summary');

  if (!summary) {
    throw new Error('Animated details requires a direct summary child.');
  }

  let expanded = details.open;
  let operation = 0;
  let closeTimer = 0;

  const setPanelInteractive = (interactive: boolean) => {
    panel.inert = !interactive;

    if (interactive) {
      panel.removeAttribute('aria-hidden');
    } else {
      panel.setAttribute('aria-hidden', 'true');
    }
  };

  const syncExpandedState = () => {
    summary.setAttribute('aria-expanded', String(expanded));
    onExpandedChange?.(expanded);
  };

  const finishClose = (currentOperation: number) => {
    if (currentOperation !== operation || expanded) {
      return;
    }

    details.open = false;
    details.dataset.detailsState = 'closed';
  };

  const open = () => {
    if (expanded) {
      return;
    }

    const currentOperation = ++operation;
    window.clearTimeout(closeTimer);
    expanded = true;
    details.open = true;
    details.dataset.detailsState = 'closed';
    setPanelInteractive(true);
    syncExpandedState();

    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      details.dataset.detailsState = 'open';
      return;
    }

    window.requestAnimationFrame(() => {
      if (currentOperation === operation && expanded) {
        details.dataset.detailsState = 'open';
      }
    });
  };

  const close = ({ immediate = false, restoreFocus = false }: CloseOptions = {}) => {
    if (!expanded) {
      return;
    }

    const currentOperation = ++operation;
    window.clearTimeout(closeTimer);
    expanded = false;
    details.dataset.detailsState = 'closed';
    setPanelInteractive(false);
    syncExpandedState();

    if (restoreFocus) {
      summary.focus();
    }

    if (immediate || window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      finishClose(currentOperation);
      return;
    }

    const transitionTime = getTransitionTime(panel);

    if (transitionTime === 0) {
      finishClose(currentOperation);
      return;
    }

    closeTimer = window.setTimeout(() => finishClose(currentOperation), transitionTime + 34);
  };

  const toggle = () => {
    if (expanded) {
      close();
    } else {
      open();
    }
  };

  const controller = { close, open, toggle };
  controllers.set(details, controller);
  details.dataset.detailsState = expanded ? 'open' : 'closed';
  setPanelInteractive(expanded);
  syncExpandedState();
  summary.addEventListener('click', (event) => {
    event.preventDefault();
    toggle();
  });

  return controller;
};
