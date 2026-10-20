import type { EventSourceLike } from "@/features/logs/hooks/use-live-logs";

// A controllable fake stream — the injected-source seam the hook exposes for
// exactly this (mirrors the backend faking PodLogStream in its tests).
export class FakeEventSource implements EventSourceLike {
  onopen: (() => void) | null = null;
  onmessage: ((ev: { data: string }) => void) | null = null;
  onerror: ((ev: { data?: unknown }) => void) | null = null;
  closed = false;
  constructor(public url: string) {}
  close() {
    this.closed = true;
  }
  // Test helpers.
  open() {
    this.onopen?.();
  }
  frame(log: unknown) {
    this.onmessage?.({ data: JSON.stringify(log) });
  }
  raw(data: string) {
    this.onmessage?.({ data });
  }
  drop() {
    this.onerror?.({});
  }
  terminate(reason: string) {
    this.onerror?.({ data: reason });
  }
}
