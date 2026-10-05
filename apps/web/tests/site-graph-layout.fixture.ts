import assert from 'node:assert/strict';
import { Worker } from 'node:worker_threads';

import { createLayoutLoop } from '../src/application/site-graph/site-graph.layout.ts';
import type {
  GraphLayoutRequest,
  GraphLayoutResponse,
} from '../src/application/site-graph/site-graph.protocol.ts';

function clock() {
  let now = 0;
  let id = 0;
  const timers = new Map<number, { callback: () => void; delay: number }>();
  return {
    now: () => now,
    schedule(callback: () => void, delay: number) {
      timers.set(++id, { callback, delay });
      return id;
    },
    clear(timer: number) {
      timers.delete(timer);
    },
    advance(duration: number) {
      now += duration;
    },
    next() {
      const entry = timers.entries().next().value;
      if (!entry) return false;
      timers.delete(entry[0]);
      now += entry[1].delay;
      entry[1].callback();
      return true;
    },
    get pending() {
      return timers.size;
    },
  };
}

export function scheduledLayout(cost: number) {
  const timer = clock();
  const updates: { progress: number; time: number }[] = [];
  const loop = createLayoutLoop({
    ...timer,
    step: () => timer.advance(cost),
    publish: (progress) => updates.push({ progress, time: timer.now() }),
    error: assert.fail,
  });
  return { timer, updates, loop };
}

export function workerHarness() {
  const url = new URL('../src/application/site-graph/site-graph.layout.worker.ts', import.meta.url);
  const source = `
    import { parentPort } from 'node:worker_threads';
    let timerID = 0;
    let time = 0;
    const timers = new Map();
    globalThis.self = {
      setTimeout(callback, delay) { timers.set(++timerID, { callback, delay }); return timerID; },
      clearTimeout(id) { timers.delete(id); },
      postMessage(message, options) { parentPort.postMessage(message, options?.transfer ?? []); }
    };
    Object.defineProperty(globalThis, 'performance', { value: { now: () => time } });
    await import(${JSON.stringify(url.href)});
    parentPort.on('message', (message) => {
      if (message.type === 'test:advance') {
        const entry = timers.entries().next().value;
        if (entry) { timers.delete(entry[0]); time += entry[1].delay; entry[1].callback(); }
      } else self.onmessage({ data: message });
      parentPort.postMessage({ type: 'test:ack', pending: timers.size });
    });
  `;
  const worker = new Worker(new URL(`data:text/javascript,${encodeURIComponent(source)}`));
  return {
    worker,
    send(message: GraphLayoutRequest | { readonly type: 'test:advance' }) {
      return new Promise<{ messages: GraphLayoutResponse[]; pending: number }>(
        (resolve, reject) => {
          const messages: GraphLayoutResponse[] = [];
          const onMessage = (
            response: GraphLayoutResponse | { type: 'test:ack'; pending: number },
          ) => {
            if (response.type !== 'test:ack') {
              messages.push(response);
              return;
            }
            worker.off('message', onMessage);
            worker.off('error', reject);
            resolve({ messages, pending: response.pending });
          };
          worker.on('message', onMessage);
          worker.once('error', reject);
          worker.postMessage(message);
        },
      );
    },
  };
}
