import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
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

const baseOpts = (
  over: Partial<UseLiveLogsOptions> = {},
): UseLiveLogsOptions => ({
  resource: "web",
  enabled: true,
  type: "all",
  text: "",
  instance: "",
  createEventSource: factory,
  ...over,
});

// The hook batches incoming frames and flushes them to state on a 100ms
// timer, so a test drives the clock to observe the flushed buffer.
const flush = () => {
  act(() => vi.advanceTimersByTime(100));
};

describe("useLiveLogs", () => {
  beforeEach(() => {
    last = null;
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
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
    flush();
    expect(result.current.lines.map((l) => l.message)).toEqual(["01", "02"]);
  });

  it("batches a burst of frames into one state flush", () => {
    const { result } = renderHook(() => useLiveLogs(baseOpts()));
    act(() => {
      last!.frame(renderLog("01"));
      last!.frame(renderLog("02"));
      last!.frame(renderLog("03"));
    });
    // Nothing rendered yet — the burst is buffered until the flush timer fires.
    expect(result.current.lines).toHaveLength(0);
    flush();
    expect(result.current.lines.map((l) => l.message)).toEqual([
      "01",
      "02",
      "03",
    ]);
  });

  it("dedupes identical frames within the stream", () => {
    const { result } = renderHook(() => useLiveLogs(baseOpts()));
    act(() => last!.frame(renderLog("01")));
    flush();
    act(() => last!.frame(renderLog("01"))); // exact duplicate (reconnect replay)
    flush();
    expect(result.current.lines).toHaveLength(1);
  });

  it("ignores a malformed frame without tearing down the tail", () => {
    const { result } = renderHook(() => useLiveLogs(baseOpts()));
    act(() => last!.raw("{not json"));
    act(() => last!.frame(renderLog("01")));
    flush();
    expect(result.current.lines.map((l) => l.message)).toEqual(["01"]);
  });

  it("caps the ring buffer, dropping the oldest lines", () => {
    const { result } = renderHook(() => useLiveLogs(baseOpts({ maxLines: 3 })));
    act(() => {
      for (let i = 1; i <= 5; i++) last!.frame(renderLog(String(i)));
    });
    flush();
    expect(result.current.lines.map((l) => l.message)).toEqual(["3", "4", "5"]);
  });

  it("re-admits a line whose key the cap evicted (dedupe set stays in sync)", () => {
    const { result } = renderHook(() => useLiveLogs(baseOpts({ maxLines: 2 })));
    act(() => {
      last!.frame(renderLog("1"));
      last!.frame(renderLog("2"));
      last!.frame(renderLog("3"));
    });
    flush();
    expect(result.current.lines.map((l) => l.message)).toEqual(["2", "3"]);

    // "1" was evicted, so its key no longer dedupes — a replay re-appends it.
    act(() => last!.frame(renderLog("1")));
    flush();
    expect(result.current.lines.map((l) => l.message)).toEqual(["3", "1"]);
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

  it("reopens after a terminal error when retryDelayMs is set, keeping the buffer", () => {
    const { result } = renderHook(() =>
      useLiveLogs(baseOpts({ retryDelayMs: 5000 })),
    );
    act(() => last!.frame(renderLog("01")));
    flush();
    const first = last!;
    // The build-tail race: the server terminates because the pod isn't up yet.
    act(() => first.terminate("no running build is available to follow"));
    expect(first.closed).toBe(true);
    expect(result.current.status).toBe("error");

    act(() => vi.advanceTimersByTime(5000));
    expect(last).not.toBe(first); // a fresh subscription opened
    expect(result.current.lines).toHaveLength(1); // buffer survived the retry

    act(() => last!.open());
    act(() => last!.frame(renderLog("02")));
    flush();
    expect(result.current.status).toBe("open");
    expect(result.current.lines.map((l) => l.message)).toEqual(["01", "02"]);
  });

  it("flushes the trailing batch into the surviving buffer on a retry", () => {
    const { result } = renderHook(() =>
      useLiveLogs(baseOpts({ retryDelayMs: 5000 })),
    );
    // A frame still pending (no flush yet) when the server terminates the tail.
    act(() => last!.frame(renderLog("01")));
    act(() => last!.terminate("no running build is available to follow"));

    act(() => vi.advanceTimersByTime(5000));
    expect(result.current.lines.map((l) => l.message)).toEqual(["01"]);
  });

  it("cancels a pending retry when the tail is disabled", () => {
    const { rerender } = renderHook((props) => useLiveLogs(props), {
      initialProps: baseOpts({ retryDelayMs: 5000 }),
    });
    const first = last!;
    act(() => first.terminate("no running build is available to follow"));

    rerender(baseOpts({ retryDelayMs: 5000, enabled: false }));
    act(() => vi.advanceTimersByTime(10000));
    expect(last).toBe(first); // no new stream after the pause
  });

  it("drops a stale tail's pending batch when the filter changes", () => {
    const { result, rerender } = renderHook((props) => useLiveLogs(props), {
      initialProps: baseOpts(),
    });
    // A frame still pending (no flush yet) at the moment the filter switches.
    act(() => last!.frame(renderLog("01")));

    rerender(baseOpts({ text: "error" }));
    flush();
    // The old filter's trailing batch must not resurrect in the fresh buffer.
    expect(result.current.lines).toHaveLength(0);
    expect(last!.url).toContain("text=error");
  });

  it("resets the buffer and reopens when the filter changes", () => {
    const { result, rerender } = renderHook((props) => useLiveLogs(props), {
      initialProps: baseOpts(),
    });
    act(() => last!.frame(renderLog("01")));
    flush();
    expect(result.current.lines).toHaveLength(1);
    const first = last!;

    rerender(baseOpts({ text: "error" }));
    expect(first.closed).toBe(true); // old stream torn down
    expect(result.current.lines).toHaveLength(0); // buffer cleared
    expect(last!.url).toContain("text=error");
  });

  it("resets dedupe across a disable/enable round trip with unchanged filters", () => {
    const { result, rerender } = renderHook((props) => useLiveLogs(props), {
      initialProps: baseOpts(),
    });
    act(() => last!.frame(renderLog("01")));
    flush();
    expect(result.current.lines).toHaveLength(1);

    rerender(baseOpts({ enabled: false }));
    rerender(baseOpts());
    // The resumed stream replays the same frame: the buffer was reset, so the
    // dedupe set must have reset with it — the replayed line re-appends.
    act(() => last!.frame(renderLog("01")));
    flush();
    expect(result.current.lines.map((l) => l.message)).toEqual(["01"]);
  });

  it("closes the stream on unmount", () => {
    const { unmount } = renderHook(() => useLiveLogs(baseOpts()));
    const es = last!;
    unmount();
    expect(es.closed).toBe(true);
  });
});
