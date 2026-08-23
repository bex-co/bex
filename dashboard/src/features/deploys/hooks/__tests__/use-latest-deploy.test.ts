import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import { useLatestDeploy } from "../use-latest-deploy";

const useQuery = vi.fn();
const startPolling = vi.fn();
const stopPolling = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => useQuery(...args),
}));

function mockDeploys(deploys: Array<{ id: string; status: string }> | null) {
  useQuery.mockReturnValue({
    data: deploys ? { deploys } : undefined,
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
});

describe("useLatestDeploy", () => {
  it("requests one newest deploy and maps only header facts", () => {
    mockDeploys([{ id: "dep-new", status: "build_failed" }]);

    const { result } = renderHook(() => useLatestDeploy("web"));

    expect(useQuery).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ variables: { serviceId: "web", limit: 1 } }),
    );
    expect(result.current.deploy).toEqual({
      id: "dep-new",
      status: "build_failed",
    });
  });

  it("returns null for an empty history", () => {
    mockDeploys([]);

    const { result } = renderHook(() => useLatestDeploy("web"));

    expect(result.current.deploy).toBeNull();
  });

  // w6/m46 t005. This is the header's DEPLOY badge, and a deploy that closes
  // server-side fires no mutation in the browser — no refetchQueries hook to
  // hang off, unlike the Cancel/Rollback button w6/m45 t003 fixed. The poll
  // interval is therefore exactly how long the badge can keep showing a status
  // the deploy has already left.
  it.each(["created", "queued", "build_in_progress", "update_in_progress"])(
    "polls at the converging interval while the deploy is %s",
    (status) => {
      mockDeploys([{ id: "dep-open", status }]);
      renderHook(() => useLatestDeploy("web"));
      expect(startPolling).toHaveBeenCalledWith(3_000);
    },
  );

  it.each(["live", "build_failed", "canceled", "update_failed", "deactivated"])(
    "falls back to the baseline once the deploy is %s",
    (status) => {
      mockDeploys([{ id: "dep-done", status }]);
      renderHook(() => useLatestDeploy("web"));
      expect(startPolling).toHaveBeenCalledWith(30_000);
    },
  );

  it("drops to the baseline as soon as a running deploy closes, with no local mutation involved", () => {
    mockDeploys([{ id: "dep-open", status: "build_in_progress" }]);
    const { rerender } = renderHook(() => useLatestDeploy("web"));
    expect(startPolling).toHaveBeenLastCalledWith(3_000);

    // The next poll response IS the server-driven closure.
    mockDeploys([{ id: "dep-open", status: "canceled" }]);
    rerender();
    expect(startPolling).toHaveBeenLastCalledWith(30_000);
  });

  it("does not restart the poll timer when nothing changed", () => {
    mockDeploys([{ id: "dep-open", status: "build_in_progress" }]);
    const { rerender } = renderHook(() => useLatestDeploy("web"));
    rerender();
    // Apollo's startPolling/stopPolling are stable across renders, so the
    // effect must not re-run and reset the interval on every render.
    expect(startPolling).toHaveBeenCalledTimes(1);
    expect(stopPolling).not.toHaveBeenCalled();
  });

  it("polls at the converging interval before the first response lands", () => {
    mockDeploys(null);
    renderHook(() => useLatestDeploy("web"));
    expect(startPolling).toHaveBeenCalledWith(3_000);
  });
});
