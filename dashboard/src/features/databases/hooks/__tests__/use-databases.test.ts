import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { useDatabases } from "@/features/databases/hooks/use-databases";

const mockUseQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

beforeEach(() => mockUseQuery.mockReset());

describe("useDatabases", () => {
  it("maps wire Databases onto normalized views and drops nulls", () => {
    mockUseQuery.mockReturnValue({
      data: {
        databases: [
          {
            __typename: "Database",
            id: "db",
            name: "db",
            plan: "free",
            version: "18",
            status: "creating",
            diskSizeGB: 1,
            suspended: "not_suspended",
            createdAt: "2026-01-01T00:00:00Z",
            public: false,
          },
          null,
        ],
      },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
      startPolling: vi.fn(),
      stopPolling: vi.fn(),
    });

    const { result } = renderHook(() => useDatabases());
    expect(result.current.databases).toHaveLength(1);
    expect(result.current.databases[0]).toMatchObject({
      id: "db",
      status: "creating",
      plan: "free",
    });
  });

  it("returns an empty list (not a crash) when data is undefined", () => {
    mockUseQuery.mockReturnValue({
      data: undefined,
      loading: true,
      error: undefined,
      refetch: vi.fn(),
      startPolling: vi.fn(),
      stopPolling: vi.fn(),
    });
    const { result } = renderHook(() => useDatabases());
    expect(result.current.databases).toEqual([]);
    expect(result.current.loading).toBe(true);
  });
});
