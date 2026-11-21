import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";

let capabilities = { canManage: false, loaded: false };
vi.mock("@/features/capabilities/hooks/use-capabilities", () => ({
  useCapabilities: () => capabilities,
}));
let currentWorkspaceId: string | null = "tea-1";
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId }),
}));

import { useWorkspaceAdmin } from "@/features/api-keys/hooks/use-workspace-admin";

beforeEach(() => {
  capabilities = { canManage: false, loaded: false };
  currentWorkspaceId = "tea-1";
});

describe("useWorkspaceAdmin (w9/m539)", () => {
  it("is unknown until the capability check first answers", () => {
    const { result } = renderHook(() => useWorkspaceAdmin());
    expect(result.current).toBeNull();
  });

  it("keeps the last answer while a refresh withdraws canManage", () => {
    capabilities = { canManage: true, loaded: true };
    const { result, rerender } = renderHook(() => useWorkspaceAdmin());
    expect(result.current).toBe(true);

    // A focus refresh: eligibility withdrawn, no answer yet.
    capabilities = { canManage: false, loaded: false };
    rerender();
    expect(result.current).toBe(true);

    // The refresh answers: a demoted Admin loses the switch.
    capabilities = { canManage: false, loaded: true };
    rerender();
    expect(result.current).toBe(false);
  });

  it("never carries one workspace's answer into another", () => {
    capabilities = { canManage: true, loaded: true };
    const { result, rerender } = renderHook(() => useWorkspaceAdmin());
    expect(result.current).toBe(true);

    currentWorkspaceId = "tea-2";
    capabilities = { canManage: false, loaded: false };
    rerender();
    expect(result.current).toBeNull();
  });
});
