import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { useServices } from "@/features/services/hooks/use-services";

const mockUseQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

beforeEach(() => mockUseQuery.mockReset());

describe("useServices", () => {
  it("maps wire Services onto normalized views and drops nulls", () => {
    mockUseQuery.mockReturnValue({
      data: {
        services: [
          {
            __typename: "Service",
            id: "app",
            name: "app",
            type: "web_service",
            suspended: "suspended",
            dashboardUrl: "https://app.onbex.co",
            url: "https://app.onbex.co",
            createdAt: "2026-01-01T00:00:00Z",
            phase: "Hibernated",
            replicas: 0,
            revision: "r1",
          },
          null,
        ],
      },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });

    const { result } = renderHook(() => useServices());

    expect(result.current.services).toHaveLength(1);
    expect(result.current.services[0]).toMatchObject({
      id: "app",
      suspended: true, // decoded from the "suspended" string enum
      phase: "Hibernated",
    });
  });

  it("returns an empty list (not a crash) when data is undefined", () => {
    mockUseQuery.mockReturnValue({
      data: undefined,
      loading: true,
      error: undefined,
      refetch: vi.fn(),
    });
    const { result } = renderHook(() => useServices());
    expect(result.current.services).toEqual([]);
    expect(result.current.loading).toBe(true);
  });

  it("polls at the baseline interval by default; poll: false mounts no timer", () => {
    mockUseQuery.mockReturnValue({
      data: undefined,
      loading: true,
      error: undefined,
      refetch: vi.fn(),
    });
    renderHook(() => useServices());
    expect(mockUseQuery).toHaveBeenLastCalledWith(
      expect.anything(),
      expect.objectContaining({ pollInterval: 30_000 }),
    );

    renderHook(() => useServices({ poll: false }));
    expect(mockUseQuery).toHaveBeenLastCalledWith(
      expect.anything(),
      expect.objectContaining({ pollInterval: 0 }),
    );
  });

  it("mounts cache-first so an SSR/prefetch-primed cache isn't refetched (w9/m62 t004)", () => {
    mockUseQuery.mockReturnValue({
      data: undefined,
      loading: true,
      error: undefined,
      refetch: vi.fn(),
    });
    renderHook(() => useServices());
    expect(mockUseQuery).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ fetchPolicy: "cache-first" }),
    );
  });
});
