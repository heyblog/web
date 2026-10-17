import { utf8Length } from '../../shared/text-length.ts';

import { type Field, fieldIssue } from './field-validation.browser.ts';
export type FieldErrors = Readonly<Record<string, string>>;
let nextFieldID = 0;

function fields(form: HTMLFormElement): Field[] {
  return Array.from(form.querySelectorAll('input, select, textarea')).filter(
    (field): field is Field =>
      (field instanceof HTMLInputElement ||
        field instanceof HTMLSelectElement ||
        field instanceof HTMLTextAreaElement) &&
      !field.matches(':disabled') &&
      field.type !== 'hidden' &&
      !(
        (field instanceof HTMLInputElement || field instanceof HTMLTextAreaElement) &&
        field.readOnly
      ),
  );
}

function renderIssue(field: HTMLElement, message: string): void {
  if (!field.id) {
    do {
      field.id = `form-field-${++nextFieldID}`;
    } while (document.querySelectorAll(`[id="${field.id}"]`).length > 1);
  }
  const errorID = `${field.id}-feedback`;
  let error = document.getElementById(errorID);
  if (!error && message) {
    error = document.createElement('p');
    error.id = errorID;
    error.className = 'text-xs/normal font-normal text-danger-fg';
    error.setAttribute('aria-live', 'polite');
    field.insertAdjacentElement('afterend', error);
  }
  if (error) {
    error.textContent = message;
    error.hidden = !message;
  }
  const descriptions = (field.getAttribute('aria-describedby') ?? '')
    .split(/\s+/u)
    .filter((id) => id && id !== errorID);
  if (message) {
    descriptions.push(errorID);
    field.setAttribute('aria-invalid', 'true');
  } else if (!field.dataset.groupFeedback) {
    field.removeAttribute('aria-invalid');
  }
  if (descriptions.length) field.setAttribute('aria-describedby', descriptions.join(' '));
  else field.removeAttribute('aria-describedby');
  if (field.hasAttribute('data-field-group')) {
    const target = field.querySelector<HTMLElement>(
      'input:not(:disabled),select:not(:disabled),textarea:not(:disabled),button:not(:disabled)',
    );
    if (target) {
      const ids = (target.getAttribute('aria-describedby') ?? '')
        .split(/\s+/u)
        .filter((id) => id && id !== errorID);
      if (message) {
        ids.push(errorID);
        target.dataset.groupFeedback = errorID;
        target.setAttribute('aria-invalid', 'true');
      } else {
        delete target.dataset.groupFeedback;
        if (!target.id || document.getElementById(`${target.id}-feedback`)?.hidden !== false)
          target.removeAttribute('aria-invalid');
      }
      if (ids.length) target.setAttribute('aria-describedby', ids.join(' '));
      else target.removeAttribute('aria-describedby');
    }
  }
}

export function showFieldIssue(form: HTMLElement, name: string, message: string): boolean {
  const field =
    Array.from(form.querySelectorAll<HTMLElement>('[data-field-group]')).find(
      (group) => group.dataset.fieldGroup === name,
    ) ??
    Array.from(form.querySelectorAll<Field>('input,select,textarea')).find(
      (input) => input.name === name,
    );
  if (!field) return false;
  renderIssue(field, message);
  field.dispatchEvent(new CustomEvent('field-feedback-error', { bubbles: true, detail: message }));
  const target = field.matches('input,select,textarea,button')
    ? field
    : field.querySelector<HTMLElement>(
        'input:not(:disabled),select:not(:disabled),textarea:not(:disabled),button:not(:disabled)',
      );
  target?.focus();
  return true;
}

/** Native fields keep their HTML fallback; this action adds persistent, accessible feedback. */
export function formFeedback(form: HTMLFormElement, initialErrors: FieldErrors = {}) {
  const touched = new Set<Field>();
  let serverErrors = { ...initialErrors };
  const nativeValidation = form.noValidate;
  let attempted = false;
  const updateField = (field: Field) => {
    renderIssue(field, serverErrors[field.name] ?? fieldIssue(field));
    const counterID = field.getAttribute('data-length-counter');
    const counter = counterID ? document.getElementById(counterID) : null;
    if (counter) counter.textContent = `当前 ${utf8Length(field.value)} 字节`;
  };
  const blur = (event: FocusEvent) => {
    const field = fields(form).find((input) => input === event.target);
    if (!field) return;
    touched.add(field);
    updateField(field);
  };
  const input = (event: Event) => {
    const changed = fields(form).find((field) => field === event.target);
    if (!changed) return;
    delete serverErrors[changed.name];
    const group = changed.closest<HTMLElement>('[data-field-group]');
    if (group) renderIssue(group, '');
    if (attempted || touched.has(changed)) updateField(changed);
    else {
      const counterID = changed.getAttribute('data-length-counter');
      const counter = counterID ? document.getElementById(counterID) : null;
      if (counter) counter.textContent = `当前 ${utf8Length(changed.value)} 字节`;
    }
    for (const field of fields(form)) {
      if (field.dataset.match === changed.name && (attempted || touched.has(field))) {
        delete serverErrors[field.name];
        updateField(field);
      }
    }
  };
  const serverError = (event: Event) => {
    const field = fields(form).find((input) => input === event.target);
    if (field && event instanceof CustomEvent && typeof event.detail === 'string') {
      touched.add(field);
      serverErrors[field.name] = event.detail;
    }
  };
  const submit = (event: SubmitEvent) => {
    attempted = true;
    let first: Field | undefined;
    for (const field of fields(form)) {
      updateField(field);
      if (!first && (serverErrors[field.name] || fieldIssue(field))) first = field;
    }
    if (first) {
      event.preventDefault();
      event.stopImmediatePropagation();
      first.focus();
    }
  };
  const applyErrors = (errors: FieldErrors) => {
    const previous = serverErrors;
    serverErrors = { ...errors };
    for (const field of fields(form)) {
      if (serverErrors[field.name]) touched.add(field);
      if (serverErrors[field.name] || previous[field.name]) updateField(field);
    }
    const first = fields(form).find((field) => serverErrors[field.name]);
    first?.focus();
  };
  form.addEventListener('focusout', blur);
  const queuedInput = (event: Event) => queueMicrotask(() => input(event));
  form.addEventListener('input', queuedInput);
  form.addEventListener('change', queuedInput);
  form.addEventListener('submit', submit, true);
  form.addEventListener('field-feedback-error', serverError);
  form.noValidate = true;
  applyErrors(initialErrors);
  return {
    update: applyErrors,
    destroy() {
      form.removeEventListener('focusout', blur);
      form.removeEventListener('input', queuedInput);
      form.removeEventListener('change', queuedInput);
      form.removeEventListener('submit', submit, true);
      form.removeEventListener('field-feedback-error', serverError);
      form.noValidate = nativeValidation;
    },
  };
}

export function initFormFeedback(): void {
  document.querySelectorAll<HTMLFormElement>('form[data-field-feedback]').forEach((form) => {
    const errors = Object.fromEntries(
      Array.from(form.querySelectorAll<HTMLInputElement>('[data-server-error]')).map((field) => [
        field.name,
        field.dataset.serverError ?? '',
      ]),
    );
    formFeedback(form, errors);
  });
}
