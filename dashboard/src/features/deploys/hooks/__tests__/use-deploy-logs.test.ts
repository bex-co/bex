import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { useDeployLogs } from "@/features/deploys/hooks/use-deploy-logs";

const mockUseQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

// useLiveLogs opens an EventSource connection — stub it to a no-op in tests.
// The deploy-logs hook merges live lines below; integration of the two is covered
// by use-live-logs.test.ts and the deploy-log-panel tests.
vi.mock("@/features/logs/hooks/use-live-logs", () => ({
  useLiveLogs: () => ({ lines: [], status: "idle" }),
}));

interface Call {
  type: string;
  skip?: boolean;
}

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
  mockUseQuery.mockImplementation((_doc: unknown, opts: Record<string, unknown>) => {
    const variables = opts.variables as { type: string };
    calls.push({ type: variables.type, skip: opts.skip as boolean | undefined });
    return {
      data: responses[variables.type] ? { logs: responses[variables.type]!.logs } : undefined,
      loading: false,
      error: undefined,
    };
  });
}

beforeEach(() => {
  mockUseQuery.mockReset();
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
      useDeployLogs("web", "2026-07-14T00:00:00Z", undefined, false),
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
          logs: [entry("2026-07-14T00:00:01.000Z", "pre-deploy ok", "predeploy")],
        },
        app: {
          logs: [entry("2026-07-14T00:00:03.000Z", "app started", "app")],
        },
      },
      calls,
    );

    const { result } = renderHook(() =>
      useDeployLogs("web", "2026-07-14T00:00:00Z", undefined, true),
    );

    expect(result.current.lines.map((l) => l.message)).toEqual([
      "==> build step 1",
      "pre-deploy ok",
      "==> build done",
      "app started",
    ]);
  });

  it("surfaces buildStoreUnavailable without erroring the whole panel", () => {
    mockUseQuery.mockImplementation((_doc: unknown, opts: Record<string, unknown>) => {
      const variables = opts.variables as { type: string };
      if (variables.type === "build") {
        return {
          data: undefined,
          loading: false,
          error: new Error("logs: the durable log store is not configured"),
        };
      }
      return { data: { logs: [] }, loading: false, error: undefined };
    });

    const { result } = renderHook(() =>
      useDeployLogs("web", "2026-07-14T00:00:00Z", undefined, true),
    );

    expect(result.current.buildStoreUnavailable).toBe(true);
    expect(result.current.error).toBeUndefined();
  });
});
