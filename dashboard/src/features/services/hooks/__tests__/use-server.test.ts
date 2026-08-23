import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { useServer } from "@/features/services/hooks/use-server";

const mockUseQuery = vi.fn();
const startPolling = vi.fn();
const stopPolling = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

const wireService = {
  __typename: "Service" as const,
  id: "app",
  name: "app",
  type: "web_service",
  suspended: "suspended", // Render's string enum, not a boolean
  dashboardUrl: "https://app.onbex.co",
  url: "https://app.onbex.co",
  createdAt: "2026-01-01T00:00:00Z",
  phase: "Hibernated",
  replicas: 0,
  revision: "r1",
};

beforeEach(() => {
  mockUseQuery.mockReset();
  startPolling.mockReset();
  stopPolling.mockReset();
});

describe("useServer", () => {
  it("maps the wire Service onto a normalized view, decoding the string suspended enum", () => {
    mockUseQuery.mockReturnValue({
      data: { server: wireService },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
      startPolling,
      stopPolling,
    });

    const { result } = renderHook(() => useServer("app"));

    expect(result.current.service).toMatchObject({
      id: "app",
      name: "app",
      suspended: true, // decoded from "suspended"
      phase: "Hibernated",
      url: "https://app.onbex.co",
      replicas: 0,
      revision: "r1",
    });
  });

  it("passes the id as a query variable", () => {
    mockUseQuery.mockReturnValue({
      data: undefined,
      loading: true,
      error: undefined,
      refetch: vi.fn(),
      startPolling,
      stopPolling,
    });
    renderHook(() => useServer("hello-go"));
    expect(mockUseQuery).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ variables: { id: "hello-go" } }),
    );
  });

  it("returns a null service (not a crash) when data is undefined", () => {
    mockUseQuery.mockReturnValue({
      data: undefined,
      loading: true,
      error: undefined,
      refetch: vi.fn(),
      startPolling,
      stopPolling,
    });
    const { result } = renderHook(() => useServer("app"));
    expect(result.current.service).toBeNull();
    expect(result.current.loading).toBe(true);
  });

  it("polls at the baseline interval once the service has settled", () => {
    mockUseQuery.mockReturnValue({
      data: { server: { ...wireService, phase: "Running" } },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
      startPolling,
      stopPolling,
    });
    renderHook(() => useServer("app"));
    expect(startPolling).toHaveBeenCalledWith(30_000);
  });

  // w6/m46 t005. A deploy that closes server-side fires no mutation in the
  // browser, so nothing refetches `server(id)` — the poll is the only thing that
  // moves the header's status pill off a phase the deploy has already left.
  // These cases never touch the Cancel/Rollback mutation (w6/m45 t003's fix);
  // they are purely about the client staying honest on its own.
  it.each(["Building", "Deploying", "Pending"])(
    "polls at the converging interval while the phase is %s",
    (phase) => {
      mockUseQuery.mockReturnValue({
        data: { server: { ...wireService, phase } },
        loading: false,
        error: undefined,
        refetch: vi.fn(),
        startPolling,
        stopPolling,
      });
      renderHook(() => useServer("app"));
      expect(startPolling).toHaveBeenCalledWith(3_000);
    },
  );

  it("drops back to the baseline as soon as the deploy closes, with no local mutation involved", () => {
    const building = {
      data: { server: { ...wireService, phase: "Building" } },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
      startPolling,
      stopPolling,
    };
    mockUseQuery.mockReturnValue(building);
    const { rerender } = renderHook(() => useServer("app"));
    expect(startPolling).toHaveBeenLastCalledWith(3_000);

    // The next poll response is the server-driven closure — a terminal phase
    // arriving with nothing in the client having asked for it.
    mockUseQuery.mockReturnValue({
      ...building,
      data: { server: { ...wireService, phase: "Failed" } },
    });
    rerender();
    expect(startPolling).toHaveBeenLastCalledWith(30_000);
  });

  it("does not restart the poll timer when nothing changed", () => {
    mockUseQuery.mockReturnValue({
      data: { server: { ...wireService, phase: "Building" } },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
      startPolling,
      stopPolling,
    });
    const { rerender } = renderHook(() => useServer("app"));
    rerender();
    // Apollo's startPolling/stopPolling are stable across renders, so the
    // effect must not re-run and reset the interval on every render.
    expect(startPolling).toHaveBeenCalledTimes(1);
    expect(stopPolling).not.toHaveBeenCalled();
  });

  it("an unloaded service polls at the converging interval, not the baseline", () => {
    mockUseQuery.mockReturnValue({
      data: undefined,
      loading: true,
      error: undefined,
      refetch: vi.fn(),
      startPolling,
      stopPolling,
    });
    renderHook(() => useServer("app"));
    expect(startPolling).toHaveBeenCalledWith(3_000);
  });

  it("poll: false mounts a secondary consumer with no poll timer of its own", () => {
    mockUseQuery.mockReturnValue({
      data: { server: { ...wireService, phase: "Building" } },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
      startPolling,
      stopPolling,
    });
    renderHook(() => useServer("app", { poll: false }));
    expect(startPolling).not.toHaveBeenCalled();
  });

  it("refetch resolves the fresh view as a one-element list (poll-to-converge shape)", async () => {
    const refetch = vi.fn().mockResolvedValue({
      data: {
        server: {
          ...wireService,
          suspended: "not_suspended",
          phase: "Running",
        },
      },
    });
    mockUseQuery.mockReturnValue({
      data: { server: wireService },
      loading: false,
      error: undefined,
      refetch,
      startPolling,
      stopPolling,
    });

    const { result } = renderHook(() => useServer("app"));
    const list = await result.current.refetch();

    expect(list).toHaveLength(1);
    expect(list[0]).toMatchObject({
      id: "app",
      suspended: false,
      phase: "Running",
    });
  });

  it("refetch resolves an empty list when the App is gone", async () => {
    const refetch = vi.fn().mockResolvedValue({ data: { server: null } });
    mockUseQuery.mockReturnValue({
      data: { server: null },
      loading: false,
      error: undefined,
      refetch,
      startPolling,
      stopPolling,
    });

    const { result } = renderHook(() => useServer("app"));
    expect(await result.current.refetch()).toEqual([]);
  });
});
