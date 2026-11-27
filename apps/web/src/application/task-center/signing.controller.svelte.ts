import type { Policy, SigningEvent, SigningKey } from '../../api/task-center/signing.types.ts';
import { signingAPI } from '../../api/task-center/task-center.browser.ts';

import { message } from './model.ts';
export class SigningController {
  policy = $state<Policy | null>(null);
  keys = $state<SigningKey[]>([]);
  events = $state<SigningEvent[]>([]);
  busy = $state(false);
  error = $state('');
  notice = $state('');
  private async load() {
    const [policy, keys, events] = await Promise.all([
      signingAPI.policy(),
      signingAPI.keys(),
      signingAPI.events(),
    ]);
    this.policy = policy;
    this.keys = keys;
    this.events = events;
  }
  private async action(work: () => Promise<void>, notice: string) {
    if (this.busy) return;
    this.busy = true;
    this.error = '';
    this.notice = '';
    try {
      await work();
      this.notice = notice;
    } catch (error) {
      this.error = message(error);
    } finally {
      this.busy = false;
    }
  }
  async refresh() {
    await this.action(() => this.load(), '');
  }
  async save(value: Policy) {
    await this.action(async () => {
      await signingAPI.savePolicy(value);
      await this.load();
    }, '签名策略已保存。');
  }
  async generate(platform: 'CF' | 'EO') {
    await this.action(async () => {
      await signingAPI.generate(platform);
      await this.load();
    }, '新密钥已生成，请先发布公钥再启用。');
  }
  async transition(key: SigningKey, action: 'publish' | 'activate' | 'revoke') {
    await this.action(async () => {
      await signingAPI.transition(key.id, action);
      await this.load();
    }, '密钥状态已更新。');
  }
}
