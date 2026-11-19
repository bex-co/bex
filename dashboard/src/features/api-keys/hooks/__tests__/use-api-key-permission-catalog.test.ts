import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";

const mockUseQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

let currentWorkspaceId: string | null = "tea-1";
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId }),
}));

import { useApiKeyPermissionCatalog } from "@/features/api-keys/hooks/use-api-key-permission-catalog";

const level = {
  __typename: "ApiKeyPermissionLevel",
  level: "read",
  name: "Read",
  grantable: true,
  requiredRole: "viewer",
  actions: [
    {
      __typename: "ApiKeyPermissionAction",
      id: "git.view",
      name: "View Git connections and repositories",
      bex: true,
    },
  ],
};
const area = (id: string, bex: boolean) => ({
  __typename: "ApiKeyPermissionArea",
  id,
  name: id,
  bex,
  permissions: [
    { __typename: "ApiKeyPermission", id, name: id, levels: [level] },
  ],
});

beforeEach(() => {
  mockUseQuery.mockReset();
  currentWorkspaceId = "tea-1";
});

describe("useApiKeyPermissionCatalog (w9/m496)", () => {
  it("reads the switcher's workspace's catalog, in bex-api's order", () => {
    const refetch = vi.fn();
    mockUseQuery.mockReturnValue({
      data: {
        apiKeyPermissionCatalog: [
          area("services", false),
          area("audit_logs", false),
          area("git", true),
        ],
      },
      error: undefined,
      refetch,
    });

    const { result } = renderHook(() => useApiKeyPermissionCatalog());

    expect(mockUseQuery).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({
        variables: { ownerId: "tea-1" },
        skip: false,
      }),
    );
    expect(result.current.areas?.map((a) => a.id)).toEqual([
      "services",
      "audit_logs",
      "git",
    ]);
    // Apollo's __typename stays out of the view.
    expect(result.current.areas?.[0].permissions[0].levels[0]).toEqual({
      level: "read",
      name: "Read",
      grantable: true,
      requiredRole: "viewer",
      actions: [
        {
          id: "git.view",
          name: "View Git connections and repositories",
          bex: true,
        },
      ],
    });
    expect(result.current.refetch).toBe(refetch);
  });

  it("narrows levels and roles: an unknown level is not offered, an unknown role is null", () => {
    mockUseQuery.mockReturnValue({
      data: {
        apiKeyPermissionCatalog: [
          {
            ...area("services", false),
            permissions: [
              {
                __typename: "ApiKeyPermission",
                id: "services",
                name: "services",
                levels: [
                  { ...level, requiredRole: "owner" },
                  { ...level, level: "admin_everything" },
                ],
              },
            ],
          },
        ],
      },
      error: undefined,
      refetch: vi.fn(),
    });

    const { result } = renderHook(() => useApiKeyPermissionCatalog());
    const levels = result.current.areas?.[0].permissions[0].levels;
    expect(levels?.map((l) => l.level)).toEqual(["read"]);
    expect(levels?.[0].requiredRole).toBeNull();
  });

  it("waits for the workspace selection before reading", () => {
    currentWorkspaceId = null;
    mockUseQuery.mockReturnValue({
      data: undefined,
      error: undefined,
      refetch: vi.fn(),
    });
    const { result } = renderHook(() => useApiKeyPermissionCatalog());
    expect(mockUseQuery).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ skip: true }),
    );
    expect(result.current.areas).toBeUndefined();
  });
});
