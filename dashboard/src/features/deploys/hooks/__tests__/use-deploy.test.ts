import { describe, it, expect, vi, beforeEach } from "vitest";
import { codedGraphQLError } from "@/test/mocks/apollo";
import { renderHook } from "@testing-library/react";
import { useDeploy } from "@/features/deploys/hooks/use-deploy";

const mockUseQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

const deploy = (over: Record<string, unknown> = {}) => ({
  id: "dep-1",
  status: "update_in_progress",
  trigger: "api",
  image: "registry.example.com/web:1",
  rollbackOf: "",
  createdAt: "2026-07-14T00:00:00Z",
  updatedAt: "2026-07-14T00:00:01Z",
  startedAt: "2026-07-14T00:00:01Z",
  finishedAt: null,
  preDeployStatus: "",
  ...over,
});

const startPolling = vi.fn();
const stopPolling = vi.fn();

beforeEach(() => {
  mockUseQuery.mockReset();
  startPolling.mockReset();
  stopPolling.mockReset();
});

describe("useDeploy", () => {
  it.each([
    "created",
    "queued",
    "build_in_progress",
    "pre_deploy_in_progress",
    "update_in_progress",
  ])("polls every 3s while the deploy is %s", (status) => {
    mockUseQuery.mockReturnValue({
      data: { deploy: deploy({ status }) },
      loading: false,
      error: undefined,
      previousData: undefined,
      startPolling,
      stopPolling,
    });

    renderHook(() => useDeploy("web", "dep-1"));

    expect(mockUseQuery).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({
        variables: { serviceId: "web", deployId: "dep-1" },
        pollInterval: 3000,
      }),
    );
    // Still in flight — the hook must not have told Apollo to stop.
    expect(stopPolling).not.toHaveBeenCalled();
  });

  // w4/262: Live closes the rollout, but a later deploy turns the record
  // Deactivated — keep reading it at the resource baseline.
  it("keeps a live deploy refreshing at the 30s baseline", () => {
    mockUseQuery.mockReturnValue({
      data: {
        deploy: deploy({ status: "live", finishedAt: "2026-07-14T00:01:00Z" }),
      },
      loading: false,
      error: undefined,
      previousData: undefined,
      startPolling,
      stopPolling,
    });

    renderHook(() => useDeploy("web", "dep-1"));

    expect(startPolling).toHaveBeenCalledWith(30_000);
    expect(stopPolling).not.toHaveBeenCalled();
  });

  it.each([
    "update_failed",
    "build_failed",
    "pre_deploy_failed",
    "canceled",
    "deactivated",
  ])("stops polling for the settled %s status", (status) => {
    mockUseQuery.mockReturnValue({
      data: { deploy: deploy({ status }) },
      loading: false,
      error: undefined,
      previousData: undefined,
      startPolling,
      stopPolling,
    });

    renderHook(() => useDeploy("web", "dep-1"));

    expect(stopPolling).toHaveBeenCalledTimes(1);
  });

  it("reports notFound once resolved with no deploy — never a phantom deploy", () => {
    mockUseQuery.mockReturnValue({
      data: { deploy: null },
      loading: false,
      error: undefined,
      previousData: undefined,
      startPolling,
      stopPolling,
    });

    const { result } = renderHook(() => useDeploy("web", "dep-nope"));

    expect(result.current.notFound).toBe(true);
    expect(result.current.deploy).toBeNull();
    expect(stopPolling).not.toHaveBeenCalled();
  });

  it("maps bex-api's GraphQL not-found error to the not-found state", () => {
    mockUseQuery.mockReturnValue({
      data: { deploy: null },
      loading: false,
      error: codedGraphQLError("NOT_FOUND", {}, 'deploy "dep-nope" not found'),
      previousData: undefined,
      startPolling,
      stopPolling,
    });

    const { result } = renderHook(() => useDeploy("web", "dep-nope"));

    expect(result.current.notFound).toBe(true);
    expect(result.current.error).toBeUndefined();
  });

  it("is loading (not notFound) on the very first render before any data arrives", () => {
    mockUseQuery.mockReturnValue({
      data: undefined,
      loading: true,
      error: undefined,
      previousData: undefined,
      startPolling,
      stopPolling,
    });

    const { result } = renderHook(() => useDeploy("web", "dep-1"));

    expect(result.current.loading).toBe(true);
    expect(result.current.notFound).toBe(false);
  });

  // w4/m198: a canceled deploy's already-running step can still settle its
  // own verdict; keep reading it for the step's bounded lifetime only.
  it("keeps polling a canceled deploy whose step may still be running, then stops", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-10-11T04:10:00Z"));
    try {
      mockUseQuery.mockReturnValue({
        data: {
          deploy: deploy({
            status: "canceled",
            preDeployStatus: "canceled",
            finishedAt: "2026-10-11T04:09:01Z",
          }),
        },
        loading: false,
        error: undefined,
        previousData: undefined,
        startPolling,
        stopPolling,
      });
      renderHook(() => useDeploy("web", "dep-1"));
      expect(stopPolling).not.toHaveBeenCalled();
      vi.advanceTimersByTime(12 * 60 * 1000);
      expect(stopPolling).toHaveBeenCalledTimes(1);
    } finally {
      vi.useRealTimers();
    }
  });

  it("stops at once for a canceled deploy whose step has its verdict", () => {
    mockUseQuery.mockReturnValue({
      data: {
        deploy: deploy({
          status: "canceled",
          preDeployStatus: "succeeded",
          finishedAt: new Date().toISOString(),
        }),
      },
      loading: false,
      error: undefined,
      previousData: undefined,
      startPolling,
      stopPolling,
    });
    renderHook(() => useDeploy("web", "dep-1"));
    expect(stopPolling).toHaveBeenCalledTimes(1);
  });
});
