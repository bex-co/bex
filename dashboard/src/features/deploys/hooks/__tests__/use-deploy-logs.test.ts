import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { useDeployLogs } from "@/features/deploys/hooks/use-deploy-logs";
import type { EventSourceLike } from "@/features/logs/hooks/use-live-logs";

const mockUseQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

interface Call {
  type: string;
  skip?: boolean;
}

class FakeEventSource implements EventSourceLike {
  onopen: (() => void) | null = null;
  onmessage: ((ev: { data: string }) => void) | null = null;
  onerror: ((ev: { data?: unknown }) => void) | null = null;
  closed = false;

  constructor(public url: string) {}

  close() {
    this.closed = true;
  }
}

let stream: FakeEventSource | null = null;
const streamFactory = (url: string) => {
  stream = new FakeEventSource(url);
  return stream;
};

const entry = (timestamp: string, message: string, type: string) => ({
  __typename: "LogEntry",
  timestamp,
  message,
  type,
  instance: null,
  level: null,
  method: null,
  statusCode: null,
});

// Route each of the three useQuery calls (build/predeploy/app) to its own
// canned response, keyed by the `type` variable — mirrors the hook firing
// three separate windowed queries (GraphQL's `type` arg is single-valued).
function stubByType(
  responses: Record<string, { logs: unknown[] } | undefined>,
  calls: Call[],
) {
  mockUseQuery.mockImplementation(
    (_doc: unknown, opts: Record<string, unknown>) => {
      const variables = opts.variables as { type: string };
      calls.push({
        type: variables.type,
        skip: opts.skip as boolean | undefined,
      });
      return {
        data: responses[variables.type]
          ? { logs: responses[variables.type]!.logs }
          : undefined,
        loading: false,
        error: undefined,
      };
    },
  );
}

beforeEach(() => {
  mockUseQuery.mockReset();
  stream = null;
});

describe("useDeployLogs", () => {
  it("windows each of the three typed queries to the deploy's resource/startTime/endTime", () => {
    const calls: Call[] = [];
    stubByType({}, calls);

    renderHook(() =>
      useDeployLogs(
        "web",
        "2026-07-14T00:00:00Z",
        "2026-07-14T00:05:00Z",
        true,
        false,
      ),
    );

    expect(mockUseQuery).toHaveBeenCalledTimes(3);
    for (const type of ["build", "predeploy", "app"]) {
      expect(mockUseQuery).toHaveBeenCalledWith(
        expect.anything(),
        expect.objectContaining({
          variables: {
            resource: "web",
            startTime: "2026-07-14T00:00:00Z",
            endTime: "2026-07-14T00:05:00Z",
            limit: 100,
            type,
          },
        }),
      );
    }
  });

  it("skips the predeploy leg when the deploy has no pre-deploy step", () => {
    const calls: Call[] = [];
    stubByType({}, calls);

    renderHook(() =>
      useDeployLogs("web", "2026-07-14T00:00:00Z", undefined, false, false),
    );

    const predeployCall = calls.find((c) => c.type === "predeploy");
    expect(predeployCall?.skip).toBe(true);
    const buildCall = calls.find((c) => c.type === "build");
    expect(buildCall?.skip).toBeFalsy();
  });

  it("interleaves build/predeploy/app lines chronologically, not grouped by type", () => {
    const calls: Call[] = [];
    stubByType(
      {
        build: {
          logs: [
            entry("2026-07-14T00:00:00.000Z", "==> build step 1", "build"),
            entry("2026-07-14T00:00:02.000Z", "==> build done", "build"),
          ],
        },
        predeploy: {
          logs: [
            entry("2026-07-14T00:00:01.000Z", "pre-deploy ok", "predeploy"),
          ],
        },
        app: {
          logs: [entry("2026-07-14T00:00:03.000Z", "app started", "app")],
        },
      },
      calls,
    );

    const { result } = renderHook(() =>
      useDeployLogs("web", "2026-07-14T00:00:00Z", undefined, true, false),
    );

    expect(result.current.lines.map((l) => l.message)).toEqual([
      "==> build step 1",
      "pre-deploy ok",
      "==> build done",
      "app started",
    ]);
  });

  it("surfaces buildStoreUnavailable without erroring the whole panel", () => {
    mockUseQuery.mockImplementation(
      (_doc: unknown, opts: Record<string, unknown>) => {
        const variables = opts.variables as { type: string };
        if (variables.type === "build") {
          return {
            data: undefined,
            loading: false,
            error: new Error("logs: the durable log store is not configured"),
          };
        }
        return { data: { logs: [] }, loading: false, error: undefined };
      },
    );

    const { result } = renderHook(() =>
      useDeployLogs("web", "2026-07-14T00:00:00Z", undefined, true, false),
    );

    expect(result.current.buildStoreUnavailable).toBe(true);
    expect(result.current.error).toBeUndefined();
  });

  it.each(["build", "predeploy", "app"])(
    "surfaces a non-store %s query failure",
    (failedType) => {
      mockUseQuery.mockImplementation(
        (_doc: unknown, opts: Record<string, unknown>) => {
          const variables = opts.variables as { type: string };
          return {
            data: variables.type === failedType ? undefined : { logs: [] },
            loading: false,
            error:
              variables.type === failedType
                ? new Error(`${failedType} query failed`)
                : undefined,
          };
        },
      );

      const { result } = renderHook(() =>
        useDeployLogs("web", "2026-07-14T00:00:00Z", undefined, true, false),
      );

      expect(result.current.error?.message).toBe(`${failedType} query failed`);
    },
  );

  it("merges the active build SSE tail into history while followBuild is enabled", () => {
    vi.useFakeTimers();
    try {
      const calls: Call[] = [];
      stubByType({}, calls);

      const { result } = renderHook(() =>
        useDeployLogs(
          "web",
          "2026-07-14T00:00:00Z",
          undefined,
          false,
          true,
          streamFactory,
        ),
      );

      expect(stream?.url).toContain("type=build");
      act(() => stream?.onopen?.());
      act(() =>
        stream?.onmessage?.({
          data: JSON.stringify({
            timestamp: "2026-07-14T00:00:01Z",
            message: "streamed build step",
            labels: [
              { name: "type", value: "build" },
              { name: "instance", value: "bld-web-gen-1-pod" },
            ],
          }),
        }),
      );
      // The tail batches frames; the 100ms flush lands them in the buffer.
      act(() => vi.advanceTimersByTime(100));

      expect(result.current.buildLiveStatus).toBe("open");
      expect(result.current.lines.map((line) => line.message)).toEqual([
        "streamed build step",
      ]);

      act(() => stream?.onerror?.({ data: "no running build" }));
      expect(result.current.buildLiveStatus).toBe("error");
      expect(stream?.closed).toBe(true);
    } finally {
      vi.useRealTimers();
    }
  });

  it("sorts a live line that predates the tail of history (ingest-lag fallback)", () => {
    vi.useFakeTimers();
    try {
      const calls: Call[] = [];
      stubByType(
        {
          build: {
            logs: [entry("2026-07-14T00:00:05.000Z", "polled line", "build")],
          },
        },
        calls,
      );

      const { result } = renderHook(() =>
        useDeployLogs(
          "web",
          "2026-07-14T00:00:00Z",
          undefined,
          false,
          true,
          streamFactory,
        ),
      );
      expect(result.current.lines.map((line) => line.message)).toEqual([
        "polled line",
      ]);

      // The stream delivers a line EARLIER than the polled history — the
      // append-only fast path would misorder it, so the full merge kicks in.
      act(() =>
        stream?.onmessage?.({
          data: JSON.stringify({
            timestamp: "2026-07-14T00:00:01Z",
            message: "late streamed line",
            labels: [
              { name: "type", value: "build" },
              { name: "instance", value: "bld-web-gen-1-pod" },
            ],
          }),
        }),
      );
      act(() => vi.advanceTimersByTime(100));

      expect(result.current.lines.map((line) => line.message)).toEqual([
        "late streamed line",
        "polled line",
      ]);
    } finally {
      vi.useRealTimers();
    }
  });

  it("appends a live line after history without re-sorting (fast path)", () => {
    vi.useFakeTimers();
    try {
      const calls: Call[] = [];
      stubByType(
        {
          build: {
            logs: [entry("2026-07-14T00:00:01.000Z", "polled line", "build")],
          },
        },
        calls,
      );

      const { result } = renderHook(() =>
        useDeployLogs(
          "web",
          "2026-07-14T00:00:00Z",
          undefined,
          false,
          true,
          streamFactory,
        ),
      );

      act(() =>
        stream?.onmessage?.({
          data: JSON.stringify({
            timestamp: "2026-07-14T00:00:05Z",
            message: "streamed line",
            labels: [
              { name: "type", value: "build" },
              { name: "instance", value: "bld-web-gen-1-pod" },
            ],
          }),
        }),
      );
      act(() => vi.advanceTimersByTime(100));

      expect(result.current.lines.map((line) => line.message)).toEqual([
        "polled line",
        "streamed line",
      ]);
    } finally {
      vi.useRealTimers();
    }
  });

  it("reopens a terminated build tail while the build is still being followed", () => {
    vi.useFakeTimers();
    try {
      const calls: Call[] = [];
      stubByType({}, calls);

      renderHook(() =>
        useDeployLogs(
          "web",
          "2026-07-14T00:00:00Z",
          undefined,
          false,
          true,
          streamFactory,
        ),
      );

      const first = stream!;
      // The subscribe-races-the-pod case: the server ends the tail before the
      // build pod exists; followBuild is still true, so the tail must retry.
      act(() => first.onerror?.({ data: "no running build" }));
      expect(first.closed).toBe(true);

      act(() => vi.advanceTimersByTime(5000));
      expect(stream).not.toBe(first);
      expect(stream?.url).toContain("type=build");
    } finally {
      vi.useRealTimers();
    }
  });

  it("keeps polling a closed window for the ingest-lag grace, then stops", () => {
    vi.useFakeTimers();
    try {
      const polls: number[] = [];
      mockUseQuery.mockImplementation(
        (_doc: unknown, opts: Record<string, unknown>) => {
          polls.push(opts.pollInterval as number);
          return { data: undefined, loading: false, error: undefined };
        },
      );

      const { rerender } = renderHook(
        ({ end }: { end?: string }) =>
          useDeployLogs("web", "2026-07-14T00:00:00Z", end, true, false),
        { initialProps: {} as { end?: string } },
      );
      expect(polls.length).toBeGreaterThan(0);
      expect(polls.every((p) => p === 5000)).toBe(true); // open window polls

      polls.length = 0;
      rerender({ end: "2026-07-14T00:05:00Z" });
      // Window just closed — the grace window still polls so store-flushed
      // build lines land without a manual reload.
      expect(polls.every((p) => p === 5000)).toBe(true);

      polls.length = 0;
      act(() => vi.advanceTimersByTime(15000));
      expect(polls.length).toBeGreaterThan(0);
      expect(polls.every((p) => p === 0)).toBe(true); // settled: polling off
    } finally {
      vi.useRealTimers();
    }
  });
});
