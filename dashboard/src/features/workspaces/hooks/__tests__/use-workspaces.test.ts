import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";

const mockUseQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

vi.mock("@/common/hooks/use-is-authenticated", () => ({
  useIsAuthenticated: () => true,
}));

import { useWorkspaces } from "@/features/workspaces/hooks/use-workspaces";

beforeEach(() => {
  mockUseQuery.mockReset();
});

describe("useWorkspaces", () => {
  it("maps workspaces to views, dropping nulls and entries without an id", () => {
    mockUseQuery.mockReturnValue({
      data: {
        workspaces: [
          {
            __typename: "Workspace",
            id: "tea-1",
            name: "acme-hq",
            plan: "hobby",
            role: "admin",
            createdAt: "2026-06-01T00:00:00Z",
          },
          null,
          { __typename: "Workspace", id: null, name: "orphan", plan: null, role: null, createdAt: null },
        ],
      },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });

    const { result } = renderHook(() => useWorkspaces());
    expect(result.current.workspaces).toEqual([
      {
        id: "tea-1",
        name: "acme-hq",
        plan: "hobby",
        role: "admin",
        createdAt: "2026-06-01T00:00:00Z",
      },
    ]);
  });

  it("defaults a missing plan to hobby rather than dropping the workspace", () => {
    mockUseQuery.mockReturnValue({
      data: {
        workspaces: [
          { __typename: "Workspace", id: "tea-1", name: "acme-hq", plan: null, role: "admin", createdAt: null },
        ],
      },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });

    const { result } = renderHook(() => useWorkspaces());
    expect(result.current.workspaces[0].plan).toBe("hobby");
  });

  it("returns an empty list (not a crash) while loading with no data yet", () => {
    mockUseQuery.mockReturnValue({
      data: undefined,
      loading: true,
      error: undefined,
      refetch: vi.fn(),
    });
    const { result } = renderHook(() => useWorkspaces());
    expect(result.current.workspaces).toEqual([]);
  });
});
