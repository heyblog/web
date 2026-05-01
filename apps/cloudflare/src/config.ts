import type { WorkerBindings } from './types';

export type WorkerConfig = {
  workerCallbackSecret?: string;
};

export function resolveWorkerConfig(bindings?: WorkerBindings): WorkerConfig {
  return {
    workerCallbackSecret: resolveWorkerCallbackSecret(bindings),
  };
}

function resolveWorkerCallbackSecret(bindings?: WorkerBindings): string | undefined {
  const bindingSecret = bindings?.WORKER_CALLBACK_SECRET?.trim();
  if (bindingSecret) {
    return bindingSecret;
  }

  if (typeof process !== 'undefined' && process.env) {
    return process.env.WORKER_CALLBACK_SECRET?.trim() || undefined;
  }

  return undefined;
}
