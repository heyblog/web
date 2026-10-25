import { OAuthProblem, requestOAuthStart } from '../../api/auth/oauth.browser.ts';
import {
  type OAuthCompletion,
  type OAuthRequest,
  oauthStartPath,
  parseOAuthCompletion,
} from '../../api/auth/oauth.types.ts';

import {
  browserEnvironment,
  type OAuthPopupEnvironment,
  type OAuthWindow,
  waitForOAuthPopup,
} from './oauth-popup-environment.browser.ts';
import { oauthMessageType } from './oauth-popup-protocol.ts';

interface PopupCallbacks {
  readonly busy: (value: boolean) => void;
  readonly fallback: (location: string | undefined) => void;
  readonly error: (code: string, fallback?: string) => void;
  readonly complete: (result: OAuthCompletion) => void;
}

interface PopupAttempt {
  popup: OAuthWindow | null;
  readonly abort: AbortController;
  readonly cleanup: (() => void)[];
}

export function createOAuthPopup(
  callbacks: PopupCallbacks,
  environment: () => OAuthPopupEnvironment = browserEnvironment,
  start = requestOAuthStart,
) {
  let active: PopupAttempt | null = null;

  function dispose() {
    const attempt = active;
    active = null;
    if (!attempt) return;
    attempt.abort.abort();
    for (const cleanup of attempt.cleanup) cleanup();
    try {
      attempt.popup?.close();
    } catch {
      // Detached browsing contexts may no longer permit window operations.
    } finally {
      callbacks.busy(false);
      callbacks.fallback(undefined);
    }
  }

  async function launch(request: OAuthRequest | (() => Promise<OAuthRequest | null>)) {
    if (active && !active.popup?.closed) {
      try {
        active.popup?.focus();
      } catch {
        dispose();
        callbacks.error('oauth_popup_unavailable');
      }
      return;
    }
    const attempt: PopupAttempt = { popup: null, abort: new AbortController(), cleanup: [] };
    let host: OAuthPopupEnvironment | undefined;
    let fallback: string | undefined;
    let requestID: string | undefined;
    let stage = 'opening';
    try {
      dispose();
      active = attempt;
      callbacks.busy(true);
      if (typeof request !== 'function') {
        fallback = oauthStartPath(request, 'redirect');
        callbacks.fallback(fallback);
      }
      host = environment();
      const origin = host.origin;
      const id = host.id();
      attempt.popup = host.open(`/auth/popup#${encodeURIComponent(id)}`);
      attempt.cleanup.push(
        host.timeout(() => {
          if (active !== attempt) return;
          dispose();
          callbacks.error('oauth_timeout', fallback);
        }, 10 * 60_000),
      );
      stage = 'starting';
      const prepare = async () => {
        const input = typeof request === 'function' ? await request() : request;
        if (active !== attempt) return null;
        if (!input) {
          dispose();
          return null;
        }
        fallback = oauthStartPath(input, 'redirect');
        callbacks.fallback(fallback);
        if (!attempt.popup) throw new OAuthProblem('oauth_popup_blocked');
        const result = await start(input, attempt.abort.signal);
        return { input, result };
      };
      const [, prepared] = await Promise.all([
        attempt.popup
          ? waitForOAuthPopup(host, attempt.popup, id, attempt.abort.signal)
          : Promise.resolve(),
        prepare(),
      ]);
      if (active !== attempt || !prepared) return;
      const { input, result } = prepared;
      requestID = result.requestID;
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
          if (event.origin === origin && event.source === attempt.popup) accept(event.data);
        }),
      );
      stage = 'listening';
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
      stage = 'navigating';
      attempt.popup?.location.replace(result.authorizationURL);
    } catch (error) {
      if (active !== attempt) return;
      const code = error instanceof OAuthProblem ? error.code : 'oauth_popup_unavailable';
      host?.report?.(stage, code, error instanceof OAuthProblem ? error.requestID : requestID);
      dispose();
      callbacks.error(code, fallback);
    }
  }
  return { launch, dispose };
}
