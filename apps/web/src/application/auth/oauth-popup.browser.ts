import { OAuthProblem, requestOAuthStart } from '../../api/auth/oauth.browser.ts';
import {
  type OAuthCompletion,
  type OAuthRequest,
  oauthStartPath,
  parseOAuthCompletion,
} from '../../api/auth/oauth.types.ts';

export interface OAuthWindow {
  readonly closed: boolean;
  readonly location: { replace(url: string): void };
  close(): void;
  focus(): void;
}

export interface OAuthPopupEnvironment {
  readonly origin: string;
  open(): OAuthWindow | null;
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
}

export const oauthMessageType = 'heyblog-oauth';
export const oauthChannelName = (state: string) => `heyblog-oauth:${state}`;

function browserEnvironment(): OAuthPopupEnvironment {
  return {
    origin: window.location.origin,
    open() {
      const width = Math.min(480, window.screen.availWidth);
      const height = Math.min(660, window.screen.availHeight);
      const left = Math.max(0, window.screenX + (window.outerWidth - width) / 2);
      const top = Math.max(0, window.screenY + (window.outerHeight - height) / 2);
      return window.open(
        'about:blank',
        '_blank',
        `popup=yes,width=${width},height=${height},left=${left},top=${top}`,
      );
    },
    listen(listener) {
      window.addEventListener('message', listener);
      return () => window.removeEventListener('message', listener);
    },
    channel(state, listener) {
      if (typeof BroadcastChannel === 'undefined') return () => {};
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
  };
}

interface PopupCallbacks {
  readonly busy: (value: boolean) => void;
  readonly error: (code: string, fallback?: string) => void;
  readonly complete: (result: OAuthCompletion) => void;
}

export function createOAuthPopup(
  callbacks: PopupCallbacks,
  environment: () => OAuthPopupEnvironment = browserEnvironment,
  start = requestOAuthStart,
) {
  let active: {
    readonly popup: OAuthWindow | null;
    readonly abort: AbortController;
    readonly cleanup: (() => void)[];
  } | null = null;
  function dispose() {
    const attempt = active;
    active = null;
    if (!attempt) return;
    attempt.abort.abort();
    for (const cleanup of attempt.cleanup) cleanup();
    attempt.popup?.close();
    callbacks.busy(false);
  }
  async function launch(request: OAuthRequest | (() => Promise<OAuthRequest | null>)) {
    if (active && !active.popup?.closed) {
      active.popup?.focus();
      return;
    }
    dispose();
    const host = environment();
    const cleanup: (() => void)[] = [];
    const attempt = { popup: host.open(), abort: new AbortController(), cleanup };
    active = attempt;
    callbacks.busy(true);
    let fallback: string | undefined;
    try {
      const input = typeof request === 'function' ? await request() : request;
      if (active !== attempt) return;
      if (!input) {
        dispose();
        return;
      }
      fallback = oauthStartPath(input, 'redirect');
      if (!attempt.popup) throw new OAuthProblem('oauth_popup_blocked');
      const result = await start(input, attempt.abort.signal);
      if (active !== attempt) return;
      const accept = (data: unknown) => {
        if (
          active !== attempt ||
          typeof data !== 'object' ||
          data === null ||
          !('type' in data) ||
          data.type !== oauthMessageType
        )
          return;
        const completed = parseOAuthCompletion(data);
        if (
          !completed ||
          completed.provider !== input.provider ||
          completed.state !== result.state ||
          completed.mode !== 'popup' ||
          (completed.outcome === 'success' &&
            (completed.intent !== input.intent || completed.claim_id !== input.claimId))
        )
          return;
        dispose();
        if (completed.outcome === 'error')
          callbacks.error(completed.error_code ?? 'request_failed', fallback);
        else callbacks.complete(completed);
      };
      attempt.cleanup.push(
        host.listen((event) => {
          if (event.origin === host.origin && event.source === attempt.popup) accept(event.data);
        }),
      );
      attempt.cleanup.push(host.channel(result.state, accept));
      let noticedClose = false;
      attempt.cleanup.push(
        host.interval(() => {
          if (attempt.popup?.closed && !noticedClose) {
            noticedClose = true;
            // COOP may detach a live popup. Keep the channel until a result or timeout.
            callbacks.busy(false);
            callbacks.error('oauth_popup_closed', fallback);
          }
        }, 500),
      );
      attempt.cleanup.push(
        host.timeout(() => {
          if (active !== attempt) return;
          dispose();
          callbacks.error('oauth_timeout', fallback);
        }, 10 * 60_000),
      );
      attempt.popup.location.replace(result.authorizationURL);
    } catch (error) {
      if (active !== attempt) return;
      dispose();
      if (error instanceof OAuthProblem) callbacks.error(error.code, fallback);
      else throw error;
    }
  }
  return { launch, dispose };
}
