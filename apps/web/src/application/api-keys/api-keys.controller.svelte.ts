import { SvelteDate } from 'svelte/reactivity';

import { ApiKeyRequestError, createApiKeyClient } from './api-keys.browser';
import type {
  ApiClientCreatePayload,
  ApiClientSummary,
  ApiClientUpdatePayload,
  ApiCredential,
  ApiKeyIssuePayload,
  ApiKeySummary,
} from './api-keys.types';

export type ClientPanel =
  | { readonly kind: 'closed' | 'create' }
  | { readonly kind: 'details' | 'edit' | 'rotate' | 'issue' | 'toggle'; readonly clientId: string }
  | { readonly kind: 'revoke'; readonly clientId: string; readonly key: ApiKeySummary }
  | { readonly kind: 'credential'; readonly clientId: string; readonly credential: ApiCredential };

export class ApiKeyController {
  clients = $state.raw<readonly ApiClientSummary[]>([]);
  panel = $state<ClientPanel>({ kind: 'closed' });
  busy = $state(false);
  refreshing = $state(false);
  error = $state<string | null>(null);
  refreshError = $state<string | null>(null);
  notice = $state<string | null>(null);
  dirty = $state(false);
  discard = $state(false);
  private api = createApiKeyClient();

  constructor(clients: readonly ApiClientSummary[]) {
    this.clients = clients;
  }

  get selected(): ApiClientSummary | undefined {
    const panel = this.panel;
    return 'clientId' in panel
      ? this.clients.find((client) => client.id === panel.clientId)
      : undefined;
  }

  open(panel: ClientPanel): void {
    if (this.busy || this.refreshing) return;
    this.panel = panel;
    this.error = null;
    this.dirty = false;
    this.discard = false;
  }

  requestClose(): void {
    if (this.busy) return;
    if (this.discard) {
      this.discard = false;
      return;
    }
    if (this.dirty) {
      this.discard = true;
      return;
    }
    this.close();
  }

  close(): void {
    const panel = this.panel;
    this.panel =
      'clientId' in panel && panel.kind !== 'details'
        ? { kind: 'details', clientId: panel.clientId }
        : { kind: 'closed' };
    this.dirty = false;
    this.discard = false;
    this.error = null;
  }

  async refresh(): Promise<void> {
    if (this.refreshing) return;
    this.refreshing = true;
    this.refreshError = null;
    try {
      this.clients = await this.api.list();
    } catch (error) {
      if (!(error instanceof ApiKeyRequestError)) throw error;
      this.refreshError = error.message;
    } finally {
      this.refreshing = false;
    }
  }

  private async mutate(operation: () => Promise<void>): Promise<void> {
    if (this.busy || this.refreshing) return;
    this.busy = true;
    this.error = null;
    this.notice = null;
    try {
      await operation();
    } catch (error) {
      if (!(error instanceof ApiKeyRequestError)) throw error;
      this.error = error.message;
    } finally {
      this.busy = false;
    }
  }

  private reveal(credential: ApiCredential): void {
    if (!this.clients.some((client) => client.id === credential.client.id)) {
      this.clients = [credential.client, ...this.clients];
    }
    this.panel = { kind: 'credential', clientId: credential.client.id, credential };
    this.dirty = false;
  }

  async create(body: ApiClientCreatePayload): Promise<void> {
    await this.mutate(async () => {
      this.reveal(await this.api.create(body));
      await this.refresh();
    });
  }

  async update(client: ApiClientSummary, body: ApiClientUpdatePayload): Promise<void> {
    await this.mutate(async () => {
      const updated = await this.api.update(client.id, body);
      this.clients = this.clients.map((current) =>
        current.id === updated.id ? { ...updated, keys: current.keys } : current,
      );
      this.panel = { kind: 'details', clientId: client.id };
      this.dirty = false;
      this.notice = '调用方设置已保存。';
      await this.refresh();
    });
  }

  async rotate(clientId: string, hours: number): Promise<void> {
    await this.mutate(async () => {
      this.reveal(await this.api.rotate(clientId, hours));
      await this.refresh();
    });
  }

  async issue(clientId: string, body: ApiKeyIssuePayload): Promise<void> {
    await this.mutate(async () => {
      this.reveal(await this.api.issue(clientId, body));
      await this.refresh();
    });
  }

  async revoke(clientId: string, key: ApiKeySummary): Promise<void> {
    await this.mutate(async () => {
      await this.api.revoke(key.id);
      this.clients = this.clients.map((client) =>
        client.id === clientId
          ? {
              ...client,
              keys: client.keys.map((current) =>
                current.id === key.id
                  ? { ...current, revoked_at: new SvelteDate().toISOString() }
                  : current,
              ),
            }
          : client,
      );
      this.panel = { kind: 'details', clientId };
      this.notice = '密钥已撤销。';
      await this.refresh();
    });
  }
}
