/** Keep Vite's literal Worker entry points outside reactive state. */
export function createGraphQueryWorker(): Worker {
  return new Worker(new URL('./site-graph.worker.ts', import.meta.url), { type: 'module' });
}

export function createGraphLayoutWorker(): Worker {
  return new Worker(new URL('./site-graph.layout.worker.ts', import.meta.url), { type: 'module' });
}
