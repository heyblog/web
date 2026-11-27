import type {
  ManualInput,
  RequestConfig,
  RequestConfigInput,
} from '../../api/task-center/configuration.types.ts';
import type { Overview, Statistics, Worker } from '../../api/task-center/observability.types.ts';
import { taskAPI, TaskTransportError } from '../../api/task-center/task-center.browser.ts';
import type {
  BatchItem,
  Control,
  Page,
  RetrySelection,
  Run,
  Schedule,
  ScheduleInput,
  TaskQuery,
} from '../../api/task-center/tasks.types.ts';

import { active, message } from './model.ts';
import { emptyPage, RunDetailsState } from './run-details.svelte.ts';

export class TasksController {
  schedules = $state<Schedule[]>([]);
  configs = $state<RequestConfig[]>([]);
  runs = $state<Run[]>([]);
  workers = $state<Worker[]>([]);
  stats = $state<Statistics[]>([]);
  overview = $state<Overview | null>(null);
  runPage = $state<Page<Run>>(emptyPage());
  readonly details = new RunDetailsState();
  checked = $state<string[]>([]);
  batchResults = $state<BatchItem[]>([]);
  query = $state<TaskQuery>({ page: 1, page_size: 20, archived: 'false' });
  statsQuery = $state<TaskQuery>({});
  busy = $state(false);
  error = $state('');
  refreshError = $state('');
  notice = $state('');
  loaded = $state(false);
  private disposed = false;
  private polling: Promise<void> | null = null;
  get shouldPoll() {
    return (
      this.workers.some((w) => w.online) ||
      this.runs.some((r) => active(r.state)) ||
      (this.details.selected !== null && active(this.details.selected.state))
    );
  }
  async refresh() {
    return this.perform(() => this.load(), '');
  }
  private async load(polling = false) {
    const [schedules, runs, configs, overview, workers, stats] = await Promise.all([
      taskAPI.schedules(),
      taskAPI.runs(this.query),
      taskAPI.configs(),
      taskAPI.overview(),
      taskAPI.workers(),
      taskAPI.stats(this.statsQuery),
    ]);
    if (this.disposed) return;
    this.schedules = schedules;
    this.runPage = runs;
    this.runs = runs.items;
    this.configs = configs;
    this.overview = overview;
    this.workers = workers;
    this.stats = stats;
    this.loaded = true;
    if (this.details.selected && (!polling || active(this.details.selected.state)))
      await this.details.load();
  }
  private async perform(action: () => Promise<void>, notice: string): Promise<boolean> {
    if (this.busy || this.disposed) return false;
    this.busy = true;
    try {
      // Finish the old read before a mutation can replace its state.
      await this.polling;
      if (this.disposed) return false;
      this.error = '';
      this.refreshError = '';
      this.notice = '';
      await action();
      this.notice = notice;
      return true;
    } catch (error) {
      this.error = message(error);
      return false;
    } finally {
      this.busy = false;
    }
  }
  private async acceptedRefresh() {
    try {
      await this.load();
    } catch (error) {
      this.refreshError = message(error);
    }
  }
  async save(value: ScheduleInput, id?: string) {
    return this.perform(async () => {
      const saved = await taskAPI.save(value, id);
      this.schedules = [...this.schedules.filter((s) => s.id !== saved.id), saved];
      await this.acceptedRefresh();
    }, '调度已保存。');
  }
  async saveConfig(value: RequestConfigInput, id?: string) {
    return this.perform(async () => {
      const saved = await taskAPI.saveConfig(value, id);
      this.configs = [...this.configs.filter((c) => c.id !== saved.id), saved];
      await this.acceptedRefresh();
    }, '请求配置已保存。');
  }
  async toggle(value: Schedule) {
    return this.save({ ...value, enabled: !value.enabled }, value.id);
  }
  async archiveSchedule(value: Schedule) {
    return this.save({ ...value, enabled: false, archived: !value.archived }, value.id);
  }
  async run(value: Schedule) {
    return this.perform(async () => {
      const id = await taskAPI.run(value.id);
      this.notice = `已创建运行 ${id}`;
      await this.acceptedRefresh();
    }, '运行已创建。');
  }
  async create(value: ManualInput, key?: string) {
    return this.perform(async () => {
      const id = await taskAPI.create(value, key);
      await this.acceptedRefresh();
      try {
        await this.details.select(await taskAPI.detail(id));
      } catch (error) {
        this.refreshError = message(error);
      }
    }, '手动任务已创建。');
  }
  async select(value: Run) {
    return this.perform(() => this.details.select(value), '');
  }
  async selectID(id: string) {
    return this.perform(async () => {
      await this.details.select(await taskAPI.detail(id));
    }, '');
  }
  async control(value: Run, action: Control) {
    return this.perform(async () => {
      let saved: Run;
      try {
        saved = await taskAPI.control(value.id, action);
      } catch (error) {
        if (error instanceof TaskTransportError && error.status === 409)
          await this.acceptedRefresh();
        throw error;
      }
      if (this.details.selected?.id === saved.id) this.details.selected = saved;
      this.runs = this.runs.map((r) => (r.id === saved.id ? saved : r));
      await this.acceptedRefresh();
    }, '操作已受理，当前状态已更新。');
  }
  async retry(value: Run, selection: RetrySelection) {
    return this.perform(async () => {
      const id = await taskAPI.retry(value.id, selection);
      await this.acceptedRefresh();
      try {
        await this.details.select(await taskAPI.detail(id));
      } catch (error) {
        this.refreshError = message(error);
      }
    }, '已沿用原参数创建新运行。');
  }
  async batch(action: Control) {
    return this.perform(async () => {
      this.batchResults = await taskAPI.batch(this.checked, action);
      this.checked = this.batchResults.filter((r) => r.code !== 'accepted').map((r) => r.id);
      await this.acceptedRefresh();
    }, '批量操作已处理，请查看逐项结果。');
  }
  async cleanup(value: Run, confirmation: string) {
    return this.perform(async () => {
      this.details.selected = await taskAPI.cleanup(value.id, confirmation);
      await this.acceptedRefresh();
    }, '运行详情已清理。');
  }
  async worker(id: string, action: 'drain' | 'resume') {
    return this.perform(async () => {
      await taskAPI.worker(id, action);
      await this.acceptedRefresh();
    }, '实例操作已受理。');
  }
  async bindVerification(id: string) {
    return this.perform(async () => {
      await taskAPI.bindVerification(id);
      await this.acceptedRefresh();
    }, '认证默认配置已更新。');
  }
  async filter(query: TaskQuery) {
    return this.perform(async () => {
      this.query = { ...query, page: 1, page_size: 20 };
      this.checked = [];
      await this.load();
    }, '');
  }
  async filterStats(query: Pick<TaskQuery, 'kind' | 'from' | 'to'>) {
    return this.perform(async () => {
      const stats = await taskAPI.stats(query);
      this.statsQuery = query;
      this.stats = stats;
    }, '');
  }
  async page(page: number) {
    return this.perform(async () => {
      this.query = { ...this.query, page };
      this.checked = [];
      await this.load();
    }, '');
  }
  async detailPage(kind: 'targets' | 'attempts' | 'events', page: number) {
    return this.perform(() => this.details.page(kind, page), '');
  }
  async download(format: 'csv' | 'json') {
    return this.perform(() => this.details.download(format), '导出已准备。');
  }
  startPolling() {
    const poll = () => {
      if (document.hidden || !this.shouldPoll || this.busy || this.polling || this.disposed) return;
      this.polling = this.load(true)
        .catch((error: unknown) => {
          if (!this.disposed) this.refreshError = message(error);
        })
        .finally(() => {
          this.polling = null;
        });
    };
    const timer = setInterval(() => {
      void poll();
    }, 3000);
    const onVisibility = () => {
      void poll();
    };
    document.addEventListener('visibilitychange', onVisibility);
    return () => {
      this.disposed = true;
      this.details.dispose();
      clearInterval(timer);
      document.removeEventListener('visibilitychange', onVisibility);
    };
  }
}
