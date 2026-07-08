import { describe, it, expect, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import {
  useLiveLogs,
  type EventSourceLike,
  type UseLiveLogsOptions,
} from "../use-live-logs";

// A controllable fake stream — the injected-source seam the hook exposes for
// exactly this (mirrors the backend faking PodLogStream in its tests).
class FakeEventSource implements EventSourceLike {
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

// The last stream the factory handed out — the effect creates it internally.
let last: FakeEventSource | null = null;
const factory = (url: string) => {
  last = new FakeEventSource(url);
  return last;
};

const renderLog = (
  message: string,
  instance = "bv1",
  timestamp = `2026-07-05T10:36:${message.padStart(2, "0")}.000Z`,
) => ({
  message,
  timestamp,
  labels: [
    { name: "type", value: "app" },
    { name: "instance", value: instance },
  ],
});

const baseOpts = (over: Partial<UseLiveLogsOptions> = {}): UseLiveLogsOptions => ({
  resource: "web",
  enabled: true,
  type: "all",
  text: "",
  createEventSource: factory,
  ...over,
});

describe("useLiveLogs", () => {
  beforeEach(() => {
    last = null;
  });

  it("stays idle and opens no stream when disabled", () => {
    const { result } = renderHook(() =>
      useLiveLogs(baseOpts({ enabled: false })),
    );
    expect(result.current.status).toBe("idle");
    expect(last).toBeNull();
  });

  it("connects, then reports open, and appends streamed lines", () => {
    const { result } = renderHook(() => useLiveLogs(baseOpts()));
    expect(result.current.status).toBe("connecting");
    expect(last?.url).toContain("/v1/logs/subscribe?resource=web");

    act(() => last!.open());
    expect(result.current.status).toBe("open");

    act(() => last!.frame(renderLog("01")));
    act(() => last!.frame(renderLog("02")));
    expect(result.current.lines.map((l) => l.message)).toEqual(["01", "02"]);
  });

  it("dedupes identical frames within the stream", () => {
    const { result } = renderHook(() => useLiveLogs(baseOpts()));
    act(() => last!.frame(renderLog("01")));
    act(() => last!.frame(renderLog("01"))); // exact duplicate (reconnect replay)
    expect(result.current.lines).toHaveLength(1);
  });

  it("ignores a malformed frame without tearing down the tail", () => {
    const { result } = renderHook(() => useLiveLogs(baseOpts()));
    act(() => last!.raw("{not json"));
    act(() => last!.frame(renderLog("01")));
    expect(result.current.lines.map((l) => l.message)).toEqual(["01"]);
  });

  it("caps the ring buffer, dropping the oldest lines", () => {
    const { result } = renderHook(() =>
      useLiveLogs(baseOpts({ maxLines: 3 })),
    );
    act(() => {
      for (let i = 1; i <= 5; i++) last!.frame(renderLog(String(i)));
    });
    expect(result.current.lines.map((l) => l.message)).toEqual(["3", "4", "5"]);
  });

  it("surfaces a transport drop as error but leaves the stream to reconnect", () => {
    const { result } = renderHook(() => useLiveLogs(baseOpts()));
    act(() => last!.drop());
    expect(result.current.status).toBe("error");
    expect(last!.closed).toBe(false); // bare drop → EventSource auto-reconnects
  });

  it("closes on a terminal error frame (server-sent reason)", () => {
    const { result } = renderHook(() => useLiveLogs(baseOpts()));
    act(() => last!.terminate("logs source not configured"));
    expect(result.current.status).toBe("error");
    expect(last!.closed).toBe(true);
  });

  it("resets the buffer and reopens when the filter changes", () => {
    const { result, rerender } = renderHook((props) => useLiveLogs(props), {
      initialProps: baseOpts(),
    });
    act(() => last!.frame(renderLog("01")));
    expect(result.current.lines).toHaveLength(1);
    const first = last!;

    rerender(baseOpts({ text: "error" }));
    expect(first.closed).toBe(true); // old stream torn down
    expect(result.current.lines).toHaveLength(0); // buffer cleared
    expect(last!.url).toContain("text=error");
  });

  it("closes the stream on unmount", () => {
    const { unmount } = renderHook(() => useLiveLogs(baseOpts()));
    const es = last!;
    unmount();
    expect(es.closed).toBe(true);
  });
});
