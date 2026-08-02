import type { LogLine } from "./types";

export type LogBufferSnapshot = {
  lines: readonly LogLine[];
  paused: boolean;
  unseen: number;
};

export class LogBuffer {
  private lines: LogLine[] = [];
  private ids = new Set<string>();
  private paused = false;
  private unseen = 0;

  constructor(private readonly capacity = 500) {
    if (!Number.isInteger(capacity) || capacity < 1) {
      throw new Error("Log buffer capacity must be a positive integer.");
    }
  }

  replace(lines: readonly LogLine[]): void {
    this.lines = [];
    this.ids.clear();
    this.unseen = 0;
    this.append(lines);
    this.unseen = 0;
  }

  append(lines: readonly LogLine[]): number {
    let added = 0;
    for (const line of lines) {
      if (this.ids.has(line.id)) continue;
      this.lines.push(line);
      this.ids.add(line.id);
      added += 1;
    }
    if (this.lines.length > this.capacity) {
      const removed = this.lines.splice(0, this.lines.length - this.capacity);
      for (const line of removed) this.ids.delete(line.id);
    }
    if (this.paused) this.unseen += added;
    return added;
  }

  setPaused(paused: boolean): void {
    this.paused = paused;
    if (!paused) this.unseen = 0;
  }

  snapshot(): LogBufferSnapshot {
    return { lines: this.lines, paused: this.paused, unseen: this.unseen };
  }
}
