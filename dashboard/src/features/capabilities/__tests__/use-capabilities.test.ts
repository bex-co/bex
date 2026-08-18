import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";

// The global test setup (src/test/setup.ts) mocks this module permissively for
// every other test; here we exercise the REAL hook.
vi.unmock("@/features/capabilities/hooks/use-capabilities");

const mockUseQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-1" }),
}));

import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";

const fullCapabilities = {
  role: "CONTRIBUTOR",
  canView: true,
  canViewLogs: true,
  canOperate: true,
  canCreate: false,
  canViewSensitive: false,
  canManageKeys: false,
  canManage: false,
  canManageBilling: false,
};

describe("useCapabilities", () => {
  beforeEach(() => mockUseQuery.mockReset());

  it("is permissive while the answer is unknown (never blocks on a stale read)", () => {
    mockUseQuery.mockReturnValue({ data: undefined, loading: true });
    const { result } = renderHook(() => useCapabilities());
    expect(result.current.canCreate).toBe(true);
    expect(result.current.canViewSensitive).toBe(true);
    expect(result.current.canManageBilling).toBe(true);
    expect(result.current.loaded).toBe(false);
    expect(result.current.loading).toBe(true);
  });

  it("reflects the server's booleans once loaded", () => {
    mockUseQuery.mockReturnValue({
      data: { viewerCapabilities: fullCapabilities },
      loading: false,
    });
    const { result } = renderHook(() => useCapabilities());
    expect(result.current.role).toBe("CONTRIBUTOR");
    expect(result.current.canOperate).toBe(true); // contributor keeps operate
    expect(result.current.canCreate).toBe(false); // ...but not create
    expect(result.current.canViewSensitive).toBe(false);
    expect(result.current.canManage).toBe(false);
    expect(result.current.loaded).toBe(true);
  });

  it("scopes the query to the active workspace", () => {
    mockUseQuery.mockReturnValue({ data: undefined, loading: false });
    renderHook(() => useCapabilities());
    const opts = mockUseQuery.mock.calls[0][1] as { variables: unknown };
    expect(opts.variables).toEqual({ ownerId: "tea-1" });
  });
});
