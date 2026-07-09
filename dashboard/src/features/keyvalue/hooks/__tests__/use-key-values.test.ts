import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { useKeyValues } from "@/features/keyvalue/hooks/use-key-values";

const mockUseQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

beforeEach(() => mockUseQuery.mockReset());

describe("useKeyValues", () => {
  it("maps wire KeyValues onto normalized views and drops nulls", () => {
    mockUseQuery.mockReturnValue({
      data: {
        keyValues: [
          {
            __typename: "KeyValue",
            id: "kv",
            name: "kv",
            plan: "free",
            version: "8",
            status: "creating",
            suspended: "not_suspended",
            createdAt: "2026-01-01T00:00:00Z",
            externalHost: null,
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

    const { result } = renderHook(() => useKeyValues());
    expect(result.current.keyValues).toHaveLength(1);
    expect(result.current.keyValues[0]).toMatchObject({
      id: "kv",
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
    const { result } = renderHook(() => useKeyValues());
    expect(result.current.keyValues).toEqual([]);
    expect(result.current.loading).toBe(true);
  });
});
