import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useDatabase } from "@/features/databases/hooks/use-database";

const mockUseQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

beforeEach(() => mockUseQuery.mockReset());

describe("useDatabase", () => {
  it("starts polling immediately after a lifecycle mutation refetch", () => {
    const refetch = vi.fn();
    const startPolling = vi.fn();
    const stopPolling = vi.fn();
    mockUseQuery.mockReturnValue({
      data: {
        database: {
          __typename: "Database",
          id: "db",
          name: "db",
          plan: "free",
          version: "17",
          status: "available",
          diskSizeGB: 1,
          suspended: "not_suspended",
          createdAt: "2026-01-01T00:00:00Z",
          public: false,
          databaseName: "db",
          databaseUser: "db_user",
          highAvailabilityEnabled: false,
          readReplicas: [],
          externalHost: null,
          backupsEnabled: false,
        },
      },
      loading: false,
      error: undefined,
      refetch,
      startPolling,
      stopPolling,
    });

    const { result } = renderHook(() => useDatabase("db"));
    startPolling.mockClear();

    act(() => result.current.refetch());

    expect(startPolling).toHaveBeenCalledWith(3000);
    expect(refetch).toHaveBeenCalledOnce();
  });
});
