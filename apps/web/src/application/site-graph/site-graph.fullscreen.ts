export type GraphFullscreenMode = 'inline' | 'page' | 'native';

export interface FullscreenPort {
  readonly nativeSupported: boolean;
  readonly isNative: () => boolean;
  readonly enterNative: () => Promise<void>;
  readonly exitNative: () => Promise<void>;
  readonly change: (mode: GraphFullscreenMode) => void;
  readonly notify: (message: string) => void;
}

/** Coalesces the key and fullscreen events belonging to one Escape press. */
export function createFullscreenEscapeGuard() {
  let blocked = false;
  return {
    press(): boolean {
      if (blocked) return false;
      blocked = true;
      return true;
    },
    nativeExited(): void {
      blocked = true;
    },
    release(): void {
      blocked = false;
    },
  };
}

/** Serializes browser promises while preserving the latest user intent. */
export function createGraphFullscreen(port: FullscreenPort) {
  let mode: GraphFullscreenMode = 'inline';
  let desired: GraphFullscreenMode = mode;
  let returnMode: 'inline' | 'page' = 'inline';
  let operation: 'enter' | 'exit' | undefined;
  let pending: Promise<void> | undefined;
  let destroyed = false;

  function commit(next: GraphFullscreenMode): void {
    if (mode === next) return;
    mode = next;
    port.change(mode);
  }

  function nonNativeMode(): 'inline' | 'page' {
    return desired === 'native' ? returnMode : desired;
  }

  async function apply(): Promise<void> {
    while (true) {
      if (port.isNative() && desired !== 'native') {
        operation = 'exit';
        try {
          await port.exitNative();
        } catch {
          // no-excuse-ok: catch -- browser policy failures stay on the UI boundary.
          if (!destroyed) {
            if (port.isNative()) {
              desired = 'native';
              commit('native');
              port.notify('未能退出原生全屏，请重试。');
            } else {
              desired = nonNativeMode();
              commit(desired);
            }
          }
          return;
        } finally {
          operation = undefined;
        }
        if (!port.isNative() && mode === 'native') commit(nonNativeMode());
        continue;
      }
      if (!port.isNative() && desired === 'native' && !destroyed) {
        operation = 'enter';
        try {
          await port.enterNative();
        } catch {
          // no-excuse-ok: catch -- never expose browser diagnostics or silently switch modes.
          desired = mode;
          if (!destroyed) port.notify('未能进入原生全屏，可重试或使用“网页全屏”。');
          return;
        } finally {
          operation = undefined;
        }
        // An Escape may have already ended fullscreen before its promise settles.
        if (!port.isNative() && desired === 'native') desired = returnMode;
        continue;
      }
      commit(desired);
      return;
    }
  }

  function reconcile(): Promise<void> {
    if (pending) return pending;
    pending = apply().finally(() => {
      pending = undefined;
      if (!destroyed && desired !== mode) return reconcile();
    });
    return pending;
  }

  function select(next: GraphFullscreenMode): Promise<void> {
    desired = next;
    if (next !== 'native' && !port.isNative() && mode !== 'native') commit(next);
    return reconcile();
  }

  return {
    nativeSupported: port.nativeSupported,
    get mode() {
      return mode;
    },
    togglePage(): Promise<void> {
      if (destroyed) return Promise.resolve();
      if (desired === 'native') returnMode = 'page';
      return select(desired === 'page' ? 'inline' : 'page');
    },
    toggleNative(): Promise<void> {
      if (destroyed) return Promise.resolve();
      if (!port.nativeSupported) {
        port.notify('浏览器不支持原生全屏，请使用“网页全屏”。');
        return Promise.resolve();
      }
      if (desired === 'native') return select(returnMode);
      returnMode = desired === 'page' ? 'page' : 'inline';
      return select('native');
    },
    escape(): Promise<void> {
      if (destroyed) return Promise.resolve();
      return select(desired === 'native' || mode === 'native' ? returnMode : 'inline');
    },
    nativeChanged(): void {
      if (destroyed || operation) return;
      if (mode === 'native' && !port.isNative()) {
        desired = returnMode;
        commit(returnMode);
      }
    },
    close(): Promise<void> {
      return select('inline');
    },
    destroy(): void {
      if (destroyed) return;
      destroyed = true;
      desired = 'inline';
      commit('inline');
      void reconcile();
    },
  };
}
