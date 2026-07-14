import { describe, it, expect, vi, beforeEach } from "vitest";
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
  startedAt: "2026-07-14T00:00:01Z",
  finishedAt: null,
  preDeployStatus: "",
  ...over,
});

const stopPolling = vi.fn();

beforeEach(() => {
  mockUseQuery.mockReset();
  stopPolling.mockReset();
});

describe("useDeploy", () => {
  it("polls every 3s while the deploy is non-terminal", () => {
    mockUseQuery.mockReturnValue({
      data: { deploy: deploy() },
      loading: false,
      error: undefined,
      previousData: undefined,
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

  it("stops polling once the deploy reaches a terminal status", () => {
    mockUseQuery.mockReturnValue({
      data: { deploy: deploy({ status: "live", finishedAt: "2026-07-14T00:01:00Z" }) },
      loading: false,
      error: undefined,
      previousData: undefined,
      stopPolling,
    });

    renderHook(() => useDeploy("web", "dep-1"));

    expect(stopPolling).toHaveBeenCalledTimes(1);
  });

  it.each(["update_failed", "canceled"])(
    "also stops polling for the %s terminal status",
    (status) => {
      mockUseQuery.mockReturnValue({
        data: { deploy: deploy({ status }) },
        loading: false,
        error: undefined,
        previousData: undefined,
        stopPolling,
      });

      renderHook(() => useDeploy("web", "dep-1"));

      expect(stopPolling).toHaveBeenCalledTimes(1);
    },
  );

  it("reports notFound once resolved with no deploy — never a phantom deploy", () => {
    mockUseQuery.mockReturnValue({
      data: { deploy: null },
      loading: false,
      error: undefined,
      previousData: undefined,
      stopPolling,
    });

    const { result } = renderHook(() => useDeploy("web", "dep-nope"));

    expect(result.current.notFound).toBe(true);
    expect(result.current.deploy).toBeNull();
    expect(stopPolling).not.toHaveBeenCalled();
  });

  it("is loading (not notFound) on the very first render before any data arrives", () => {
    mockUseQuery.mockReturnValue({
      data: undefined,
      loading: true,
      error: undefined,
      previousData: undefined,
      stopPolling,
    });

    const { result } = renderHook(() => useDeploy("web", "dep-1"));

    expect(result.current.loading).toBe(true);
    expect(result.current.notFound).toBe(false);
  });
});
