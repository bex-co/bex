import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import { mergeDeployPages, useDeploys, type DeployRow } from "../use-deploys";

const useQuery = vi.fn();
const startPolling = vi.fn();
const stopPolling = vi.fn();
const query = vi.fn();
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
    ...partial,
  };
}

function mockDeploys(
  deploys: Array<{ id: string; status: string }> | null,
) {
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
});

describe("mergeDeployPages", () => {
  it("appends without duplicates", () => {
    const { rows, dropAppended } = mergeDeployPages(
      [row({ id: "dep-1" }), row({ id: "dep-2" })],
      [row({ id: "dep-3" })],
    );
    expect(dropAppended).toBe(false);
    expect(rows.map((d) => d.id)).toEqual(["dep-1", "dep-2", "dep-3"]);
  });

  it("drops appended pages when a poll shifts page 1 into their territory", () => {
    const { rows, dropAppended } = mergeDeployPages(
      [row({ id: "dep-new" }), row({ id: "dep-1" })],
      [row({ id: "dep-1" }), row({ id: "dep-2" })],
    );
    expect(dropAppended).toBe(true);
    expect(rows.map((d) => d.id)).toEqual(["dep-new", "dep-1"]);
  });
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
