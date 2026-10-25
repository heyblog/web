import { OAuthProblem } from '../../api/auth/oauth.browser.ts';

import { isOAuthPopupReady, oauthChannelName } from './oauth-popup-protocol.ts';

export interface OAuthWindow {
  readonly closed: boolean;
  readonly location: { replace(url: string): void };
  close(): void;
  focus(): void;
}

export interface OAuthPopupEnvironment {
  readonly origin: string;
  id(): string;
  open(url: string): OAuthWindow | null;
  listen(
    listener: (event: {
      readonly origin: string;
      readonly source: unknown;
      readonly data: unknown;
    }) => void,
  ): () => void;
  channel(state: string, listener: (data: unknown) => void): () => void;
  interval(callback: () => void, milliseconds: number): () => void;
  timeout(callback: () => void, milliseconds: number): () => void;
  report?(stage: string, code: string, requestID?: string): void;
}

export function browserEnvironment(): OAuthPopupEnvironment {
  return {
    origin: window.location.origin,
    id: () => crypto.randomUUID(),
    open(url) {
      const width = Math.min(480, window.screen.availWidth);
      const height = Math.min(660, window.screen.availHeight);
      const left = Math.max(0, window.screenX + (window.outerWidth - width) / 2);
      const top = Math.max(0, window.screenY + (window.outerHeight - height) / 2);
      return window.open(
        url,
        '_blank',
        `popup=yes,width=${width},height=${height},left=${left},top=${top}`,
      );
    },
    listen(listener) {
      window.addEventListener('message', listener);
      return () => window.removeEventListener('message', listener);
    },
    channel(state, listener) {
      if (typeof BroadcastChannel === 'undefined')
        throw new OAuthProblem('oauth_popup_unavailable');
      const channel = new BroadcastChannel(oauthChannelName(state));
      channel.onmessage = (event: MessageEvent<unknown>) => listener(event.data);
      return () => channel.close();
    },
    interval(callback, milliseconds) {
      const timer = window.setInterval(callback, milliseconds);
      return () => window.clearInterval(timer);
    },
    timeout(callback, milliseconds) {
      const timer = window.setTimeout(callback, milliseconds);
      return () => window.clearTimeout(timer);
    },
    report(stage, code, requestID) {
      console.warn('OAuth popup failed', { stage, code, request_id: requestID });
    },
  };
}

export function waitForOAuthPopup(
  host: OAuthPopupEnvironment,
  popup: OAuthWindow,
  attempt: string,
  signal: AbortSignal,
): Promise<void> {
  return new Promise((resolve, reject) => {
    const cleanup: (() => void)[] = [];
    let settled = false;
    const finish = (error?: unknown) => {
      if (settled) return;
      settled = true;
      for (const stop of cleanup) stop();
      if (error) reject(error);
      else resolve();
    };
    try {
      cleanup.push(
        host.listen((event) => {
          if (
            event.origin === host.origin &&
            event.source === popup &&
            isOAuthPopupReady(event.data, attempt)
          )
            finish();
        }),
      );
      cleanup.push(host.timeout(() => finish(new OAuthProblem('oauth_popup_unavailable')), 10_000));
      const cancelled = () => finish(new OAuthProblem('oauth_cancelled'));
      signal.addEventListener('abort', cancelled, { once: true });
      cleanup.push(() => signal.removeEventListener('abort', cancelled));
      if (signal.aborted) cancelled();
    } catch (error) {
      finish(error);
    }
  });
}
