import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";

const mockUseMutation = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: (...args: unknown[]) => mockUseMutation(...args),
}));

const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: {
    success: (...a: unknown[]) => toastSuccess(...a),
    error: (...a: unknown[]) => toastError(...a),
  },
}));

// The revoked key must belong to the switcher's selection (w6/m18) — the
// backend refuses to revoke another workspace's key.
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-1" }),
}));

import { useRevokeApiKey } from "@/features/api-keys/hooks/use-revoke-api-key";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useRevokeApiKey", () => {
  it("revokes, toasts success, and resolves true", async () => {
    const mutate = vi.fn().mockResolvedValue({ data: { revokeApiKey: true } });
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useRevokeApiKey());
    let ok;
    await act(async () => {
      ok = await result.current.revoke("key-1", "deploy-agent");
    });

    expect(ok).toBe(true);
    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "key-1", ownerId: "tea-1" },
    });
    expect(toastSuccess).toHaveBeenCalledWith("Revoked deploy-agent");
  });

  it("on failure toasts an error and resolves false — caller must not drop the row (t006)", async () => {
    const mutate = vi.fn().mockRejectedValue(new Error("forbidden"));
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useRevokeApiKey());
    let ok;
    await act(async () => {
      ok = await result.current.revoke("key-1", "deploy-agent");
    });

    expect(ok).toBe(false);
    expect(toastError).toHaveBeenCalledWith("Couldn't revoke deploy-agent");
  });

  it("clears the revoking id after settling, win or lose", async () => {
    const mutate = vi.fn().mockResolvedValue({ data: { revokeApiKey: true } });
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useRevokeApiKey());
    await act(async () => {
      await result.current.revoke("key-1", "deploy-agent");
    });

    expect(result.current.revoking).toBeNull();
  });
});
