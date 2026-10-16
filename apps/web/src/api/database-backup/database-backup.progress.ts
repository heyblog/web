/** Rolling transfer rate shared by browser upload and server-preparation polling. */
export class BackupTransferMeter {
  private samples: { time: number; bytes: number }[] = [];
  record(bytes: number): number {
    const time = performance.now();
    this.samples.push({ time, bytes });
    while (this.samples.length > 1 && (this.samples[1]?.time ?? time) < time - 3000)
      this.samples.shift();
    const first = this.samples[0];
    return first && time > first.time
      ? (Math.max(0, bytes - first.bytes) * 1000) / (time - first.time)
      : 0;
  }
  reset() {
    this.samples = [];
  }
}
