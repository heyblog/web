interface Pointer {
  readonly x: number;
  readonly y: number;
}

/** A gesture stays a drag even when a pointer returns to its starting position. */
export function createGraphGesture() {
  const pointers = new Map<number, Pointer>();
  let moved = false;
  let multiple = false;
  return {
    down(id: number, point: Pointer): void {
      if (pointers.size === 0) {
        moved = false;
        multiple = false;
      }
      pointers.set(id, point);
      if (pointers.size > 1) multiple = true;
    },
    move(id: number, point: Pointer): void {
      const origin = pointers.get(id);
      if (origin && Math.hypot(point.x - origin.x, point.y - origin.y) > 6) moved = true;
    },
    up(id: number, point: Pointer): boolean {
      this.move(id, point);
      const existed = pointers.delete(id);
      return existed && pointers.size === 0 && !moved && !multiple;
    },
    cancel(): void {
      moved = true;
      pointers.clear();
    },
  };
}
