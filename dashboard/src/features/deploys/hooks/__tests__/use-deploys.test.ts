import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { useDeploys, type DeployRow } from "../use-deploys";

const useQuery = vi.fn();
const startPolling = vi.fn();
const stopPolling = vi.fn();
const query = vi.fn();
const toastError = vi.hoisted(() => vi.fn());
vi.mock("sonner", () => ({ toast: { error: toastError } }));
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => useQuery(...args),
  useApolloClient: () => ({ query }),
}));

function row(partial: Partial<DeployRow> & { id: string }): DeployRow {
  return {
    status: "live",
    trigger: "api",
    image: "",
    rollbackOf: "",
    commitId: "",
    commitMessage: "",
    commitCreatedAt: null,
    createdAt: null,
    updatedAt: null,
    startedAt: null,
    finishedAt: null,
    preDeployStatus: "",
    failureReason: "",
    cancelReason: "",
    stallReason: "",
    ...partial,
  };
}

function mockDeploys(deploys: Array<{ id: string; status: string }> | null) {
  useQuery.mockReturnValue({
    data: deploys
      ? {
          deploys: deploys.map((d) => ({
            id: d.id,
            status: d.status,
            trigger: "api",
          })),
        }
      : undefined,
    loading: deploys === null,
    error: undefined,
    startPolling,
    stopPolling,
  });
}

beforeEach(() => {
  useQuery.mockReset();
  startPolling.mockReset();
  stopPolling.mockReset();
  query.mockReset();
  toastError.mockReset();
});

describe("useDeploys", () => {
  it("requests a page of 20 and maps deploy rows", () => {
    mockDeploys([{ id: "dep-a", status: "live" }]);

    const { result } = renderHook(() => useDeploys("srv-1", []));

    expect(useQuery).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({
        variables: { serviceId: "srv-1", status: undefined, limit: 20 },
        skipPollAttempt: expect.any(Function),
      }),
    );
    expect(result.current.deploys).toEqual([
      expect.objectContaining({ id: "dep-a", status: "live" }),
    ]);
  });

  it("drops appended history and its paging state when a poll overlaps it", async () => {
    const firstPage = Array.from({ length: 20 }, (_, i) =>
      row({ id: `dep-${i}` }),
    );
    mockDeploys(firstPage);
    query.mockResolvedValue({
      data: { deploys: [row({ id: "dep-tail" })] },
      error: new Error("partial page"),
    });
    const { result, rerender } = renderHook(() => useDeploys("srv-1", []));

    act(() => result.current.loadMore());
    await waitFor(() => expect(result.current.deploys).toHaveLength(21));
    expect(result.current.hasMore).toBe(false);
    // The partial page is kept, and its error announced.
    expect(toastError).toHaveBeenCalledWith("Couldn't load older deploys.");

    const refreshed = [row({ id: "dep-tail" }), ...firstPage.slice(0, 19)];
    mockDeploys(refreshed);
    rerender();

    expect(result.current.deploys.map((deploy) => deploy.id)).toEqual(
      refreshed.map((deploy) => deploy.id),
    );
    expect(result.current.hasMore).toBe(true);
  });

  it("drops appended history when the status filter changes", async () => {
    const firstPage = Array.from({ length: 20 }, (_, i) =>
      row({ id: `dep-${i}` }),
    );
    mockDeploys(firstPage);
    query.mockResolvedValue({
      data: { deploys: [row({ id: "dep-tail" })] },
      error: new Error("old filter error"),
    });
    const { result, rerender } = renderHook(
      ({ statuses }) => useDeploys("srv-1", statuses),
      { initialProps: { statuses: [] as string[] } },
    );

    act(() => result.current.loadMore());
    await waitFor(() => expect(result.current.deploys).toHaveLength(21));

    mockDeploys([row({ id: "dep-failed", status: "build_failed" })]);
    rerender({ statuses: ["build_failed"] });

    expect(result.current.deploys.map((deploy) => deploy.id)).toEqual([
      "dep-failed",
    ]);
    expect(result.current.hasMore).toBe(false);
  });

  // w4/073 — mirror of useLatestDeploy / w6/m46 t005. Settled history still
  // polls at the baseline so a deploy started elsewhere can appear.
  it.each(["created", "queued", "build_in_progress", "update_in_progress"])(
    "polls at the converging interval while a listed deploy is %s",
    (status) => {
      mockDeploys([{ id: "dep-open", status }]);
      renderHook(() => useDeploys("srv-1", []));
      expect(startPolling).toHaveBeenCalledWith(3_000);
    },
  );

  it.each(["live", "build_failed", "canceled", "update_failed", "deactivated"])(
    "falls back to the baseline once every listed deploy is %s",
    (status) => {
      mockDeploys([{ id: "dep-done", status }]);
      renderHook(() => useDeploys("srv-1", []));
      expect(startPolling).toHaveBeenCalledWith(30_000);
    },
  );

  it("polls at the converging interval before the first response lands", () => {
    mockDeploys(null);
    renderHook(() => useDeploys("srv-1", []));
    expect(startPolling).toHaveBeenCalledWith(3_000);
  });

  it("does not restart the poll timer when nothing changed", () => {
    mockDeploys([{ id: "dep-open", status: "build_in_progress" }]);
    const { rerender } = renderHook(() => useDeploys("srv-1", []));
    rerender();
    expect(startPolling).toHaveBeenCalledTimes(1);
    expect(stopPolling).not.toHaveBeenCalled();
  });
});
