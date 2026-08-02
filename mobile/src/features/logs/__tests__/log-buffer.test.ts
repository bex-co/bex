import { LogBuffer } from "../log-buffer";
import type { LogLine } from "../types";

const line = (id: string): LogLine => ({
  id,
  message: id,
  timestamp: `2026-08-02T00:00:0${id}.000Z`,
  labels: [],
});

describe("LogBuffer", () => {
  it("deduplicates the history/live overlap and caps the oldest lines", () => {
    const subject = new LogBuffer(3);
    subject.replace([line("1"), line("2")]);
    expect(subject.append([line("2"), line("3"), line("4")])).toBe(2);
    expect(subject.snapshot().lines.map(({ id }) => id)).toEqual([
      "2",
      "3",
      "4",
    ]);
  });

  it("keeps collecting while paused and reports unseen lines", () => {
    const subject = new LogBuffer(3);
    subject.replace([line("1")]);
    subject.setPaused(true);
    subject.append([line("2"), line("2"), line("3")]);
    expect(subject.snapshot().unseen).toBe(2);
    subject.setPaused(false);
    expect(subject.snapshot().unseen).toBe(0);
  });
});
