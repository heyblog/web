import type { Attempt, TaskEvent } from '../../api/task-center/observability.types.ts';
import { taskAPI } from '../../api/task-center/task-center.browser.ts';
import type { Page, Run, Target } from '../../api/task-center/tasks.types.ts';

export function emptyPage<T>(): Page<T> {
  return { items: [], page: 1, page_size: 20, total: 0 };
}

export class RunDetailsState {
  selected = $state<Run | null>(null);
  targetPage = $state<Page<Target>>(emptyPage());
  attemptPage = $state<Page<Attempt>>(emptyPage());
  eventPage = $state<Page<TaskEvent>>(emptyPage());
  targetState = $state('');
  private disposed = false;

  async select(value: Run) {
    if (this.disposed) return;
    this.selected = value;
    this.targetPage = emptyPage();
    this.attemptPage = emptyPage();
    this.eventPage = emptyPage();
    this.targetState = '';
    await this.load();
  }

  async load() {
    if (!this.selected) return;
    const id = this.selected.id;
    const [run, targets, attempts, events] = await Promise.all([
      taskAPI.detail(id),
      taskAPI.targets(id, { page: this.targetPage.page, state: this.targetState }),
      taskAPI.attempts(id, { page: this.attemptPage.page }),
      taskAPI.events(id, { page: this.eventPage.page }),
    ]);
    if (this.disposed || this.selected?.id !== id) return;
    this.selected = run;
    this.targetPage = targets;
    this.attemptPage = attempts;
    this.eventPage = events;
  }

  async page(kind: 'targets' | 'attempts' | 'events', page: number) {
    if (!this.selected) return;
    const id = this.selected.id;
    switch (kind) {
      case 'targets': {
        const targets = await taskAPI.targets(id, { page, state: this.targetState });
        if (!this.disposed && this.selected?.id === id) this.targetPage = targets;
        break;
      }
      case 'attempts': {
        const attempts = await taskAPI.attempts(id, { page });
        if (!this.disposed && this.selected?.id === id) this.attemptPage = attempts;
        break;
      }
      case 'events': {
        const events = await taskAPI.events(id, { page });
        if (!this.disposed && this.selected?.id === id) this.eventPage = events;
        break;
      }
    }
  }

  async download(format: 'csv' | 'json') {
    if (!this.selected) return;
    const id = this.selected.id;
    const blob = await taskAPI.export(id, format, { state: this.targetState });
    if (this.disposed) return;
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `task-${id}.${format}`;
    link.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  }

  dispose() {
    this.disposed = true;
    this.selected = null;
  }
}
