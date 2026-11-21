import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";

const mockUseQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

// Scoped to the switcher's selection (w6/m18), never a workspace the hook
// resolves itself — same seam useServices uses.
let currentWorkspaceId: string | null = "tea-1";
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId }),
}));

import { useApiKeys } from "@/features/api-keys/hooks/use-api-keys";

beforeEach(() => {
  mockUseQuery.mockReset();
  currentWorkspaceId = "tea-1";
});

describe("useApiKeys", () => {
  it("maps apiKeys to views, dropping nulls and keys without an id", () => {
    mockUseQuery.mockReturnValue({
      data: {
        apiKeys: [
          {
            __typename: "ApiKey",
            id: "key-1",
            name: "deploy-agent",
            createdAt: "2026-07-01T00:00:00Z",
            createdBy: "user:minter",
            lastUsedAt: "2026-07-05T00:00:00Z",
            permissions: {
              __typename: "ApiKeyPermissions",
              all: false,
              grants: [
                {
                  __typename: "ApiKeyGrant",
                  permission: "services",
                  level: "read",
                },
              ],
            },
          },
          null,
          {
            __typename: "ApiKey",
            id: null,
            name: "orphan",
            createdAt: null,
            createdBy: null,
            lastUsedAt: null,
            permissions: {
              __typename: "ApiKeyPermissions",
              all: true,
              grants: [],
            },
          },
        ],
      },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });

    const { result } = renderHook(() => useApiKeys());
    expect(result.current.keys).toEqual([
      {
        id: "key-1",
        name: "deploy-agent",
        createdAt: "2026-07-01T00:00:00Z",
        createdBy: "user:minter",
        lastUsedAt: "2026-07-05T00:00:00Z",
        // w9/m496: the key's grant, without Apollo's __typename.
        permissions: {
          all: false,
          grants: [{ permission: "services", level: "read" }],
        },
      },
    ]);
  });

  it("never requests or surfaces a secret field — the view type has none", () => {
    mockUseQuery.mockReturnValue({
      data: {
        apiKeys: [
          {
            __typename: "ApiKey",
            id: "key-1",
            name: "x",
            createdAt: null,
            createdBy: null,
            lastUsedAt: null,
            permissions: {
              __typename: "ApiKeyPermissions",
              all: true,
              grants: [],
            },
          },
        ],
      },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });
    const { result } = renderHook(() => useApiKeys());
    expect(result.current.keys[0]).not.toHaveProperty("secret");
  });

  it("returns an empty list (not a crash) while loading with no data yet", () => {
    mockUseQuery.mockReturnValue({
      data: undefined,
      loading: true,
      error: undefined,
      refetch: vi.fn(),
    });
    const { result } = renderHook(() => useApiKeys());
    expect(result.current.keys).toEqual([]);
  });

  it("sends the switcher's workspace as ownerId, skipped until it resolves", () => {
    currentWorkspaceId = null;
    mockUseQuery.mockReturnValue({
      data: undefined,
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });

    const { result } = renderHook(() => useApiKeys());

    expect(mockUseQuery).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ variables: { ownerId: null }, skip: true }),
    );
    // Skip means loading stays true regardless of Apollo's own flag, so the
    // list page doesn't flash an empty state before the selection resolves.
    expect(result.current.loading).toBe(true);
  });

  it("asks for every member's keys only when allMembers is set (w9/m539)", () => {
    mockUseQuery.mockReturnValue({
      data: undefined,
      loading: true,
      error: undefined,
      refetch: vi.fn(),
    });
    renderHook(() => useApiKeys());
    expect(mockUseQuery).toHaveBeenLastCalledWith(
      expect.anything(),
      expect.objectContaining({ variables: { ownerId: "tea-1" } }),
    );
    renderHook(() => useApiKeys(true));
    expect(mockUseQuery).toHaveBeenLastCalledWith(
      expect.anything(),
      expect.objectContaining({
        variables: { ownerId: "tea-1", allMembers: true },
      }),
    );
  });
});
