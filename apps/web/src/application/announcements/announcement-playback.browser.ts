export interface PlaybackTimers {
  set: (callback: () => void, delay: number) => ReturnType<typeof setTimeout>;
  clear: (timer: ReturnType<typeof setTimeout>) => void;
}

export function createAnnouncementPlayback(
  count: number,
  change: (index: number) => void,
  timers: PlaybackTimers = {
    set: (callback, delay) => setTimeout(callback, delay),
    clear: (timer) => clearTimeout(timer),
  },
) {
  let index = 0;
  let stopped = false;
  let timer: ReturnType<typeof setTimeout> | undefined;
  const paused = new Set<string>();
  function schedule() {
    if (timer !== undefined) timers.clear(timer);
    timer = undefined;
    if (stopped || count < 2 || paused.size > 0) return;
    timer = timers.set(() => move(1), 6_000);
  }
  function move(direction: number) {
    if (stopped || count < 1) return;
    index = (index + direction + count) % count;
    change(index);
    schedule();
  }
  return {
    move,
    pause(reason: string, value: boolean) {
      if (value) paused.add(reason);
      else paused.delete(reason);
      schedule();
    },
    start: schedule,
    dispose() {
      stopped = true;
      if (timer !== undefined) timers.clear(timer);
      timer = undefined;
    },
  };
}
