import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { useServer } from "@/features/services/hooks/use-server";

const mockUseQuery = vi.fn();
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

beforeEach(() => mockUseQuery.mockReset());

describe("useServer", () => {
  it("maps the wire Service onto a normalized view, decoding the string suspended enum", () => {
    mockUseQuery.mockReturnValue({
      data: { server: wireService },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
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
    });
    const { result } = renderHook(() => useServer("app"));
    expect(result.current.service).toBeNull();
    expect(result.current.loading).toBe(true);
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
    });

    const { result } = renderHook(() => useServer("app"));
    expect(await result.current.refetch()).toEqual([]);
  });
});
