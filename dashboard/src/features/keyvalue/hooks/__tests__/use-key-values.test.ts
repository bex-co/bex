import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";

const mockUseQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

// The list's owner is the switcher's selection (WorkspaceProvider), never a
// workspace the hook resolves itself — see the workspace-scoping cases below.
let currentWorkspaceId: string | null = "tea-1";
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId }),
}));

import { useKeyValues } from "@/features/keyvalue/hooks/use-key-values";

/** The `ownerId` the hook most recently asked Apollo to query with. */
function lastOwnerId(): unknown {
  const calls = mockUseQuery.mock.calls;
  const [, options] = calls[calls.length - 1] as [
    unknown,
    { variables: { ownerId: string | null } },
  ];
  return options.variables.ownerId;
}

function lastSkip(): boolean {
  const calls = mockUseQuery.mock.calls;
  const [, options] = calls[calls.length - 1] as [unknown, { skip: boolean }];
  return options.skip;
}

beforeEach(() => {
  mockUseQuery.mockReset();
  currentWorkspaceId = "tea-1";
});

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

  // w6/m14 — the bug: createKeyValue already wrote into the switcher's
  // workspace, but this list query carried no ownerId at all, so a store
  // created in workspace B showed up in a list of *every* workspace's stores.
  describe("workspace scoping (w6/m14 regression)", () => {
    it("queries the switcher's selected workspace", () => {
      mockUseQuery.mockReturnValue({
        data: undefined,
        loading: true,
        error: undefined,
        refetch: vi.fn(),
        startPolling: vi.fn(),
        stopPolling: vi.fn(),
      });
      currentWorkspaceId = "tea-2";

      renderHook(() => useKeyValues());

      expect(lastOwnerId()).toBe("tea-2");
      expect(lastSkip()).toBe(false);
    });

    it("re-queries with the new ownerId after a workspace switch", () => {
      mockUseQuery.mockReturnValue({
        data: { keyValues: [] },
        loading: false,
        error: undefined,
        refetch: vi.fn(),
        startPolling: vi.fn(),
        stopPolling: vi.fn(),
      });

      const { rerender } = renderHook(() => useKeyValues());
      expect(lastOwnerId()).toBe("tea-1");

      // The switcher moves to the other workspace.
      currentWorkspaceId = "tea-2";
      rerender();

      expect(lastOwnerId()).toBe("tea-2");
      expect(lastSkip()).toBe(false);
    });

    it("skips the query until the switcher's selection resolves", () => {
      mockUseQuery.mockReturnValue({
        data: undefined,
        loading: false,
        error: undefined,
        refetch: vi.fn(),
        startPolling: vi.fn(),
        stopPolling: vi.fn(),
      });
      currentWorkspaceId = null;

      const { result } = renderHook(() => useKeyValues());

      expect(lastSkip()).toBe(true);
      // Mirrors useServices/useDatabases: no flash of the empty state before
      // the id lands.
      expect(result.current.loading).toBe(true);
      expect(result.current.keyValues).toEqual([]);
    });
  });
});
