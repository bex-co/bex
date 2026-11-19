import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { codedGraphQLError } from "@/test/mocks/apollo";

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

let currentWorkspaceId: string | null = "tea-1";
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId }),
}));

import { useUpdateApiKeyPermissions } from "@/features/api-keys/hooks/use-update-api-key-permissions";

const GRANTS = { grants: [{ permission: "services", level: "read_write" }] };

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
  currentWorkspaceId = "tea-1";
});

describe("useUpdateApiKeyPermissions (w9/m496)", () => {
  it("sends the key, its workspace and the new grant, toasts, and resolves true", async () => {
    const mutate = vi.fn().mockResolvedValue({
      data: {
        updateApiKeyPermissions: {
          id: "key-1",
          permissions: { all: false, grants: GRANTS.grants },
        },
      },
    });
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useUpdateApiKeyPermissions());
    let ok;
    await act(async () => {
      ok = await result.current.update("key-1", "deploy-agent", GRANTS);
    });

    expect(ok).toBe(true);
    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "key-1", ownerId: "tea-1", permissions: GRANTS },
    });
    expect(toastSuccess).toHaveBeenCalledWith(
      "Updated permissions for deploy-agent",
    );
    expect(result.current.busy).toBe(false);
  });

  it("surfaces the server's refusal in its own words and resolves false", async () => {
    const refusal = codedGraphQLError(
      "api_key_permissions_editor_forbidden",
      {},
      "only the key's creator or a workspace Admin may change its permissions",
    );
    const mutate = vi.fn().mockRejectedValue(refusal);
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useUpdateApiKeyPermissions());
    let ok;
    await act(async () => {
      ok = await result.current.update("key-1", "deploy-agent", GRANTS);
    });

    expect(ok).toBe(false);
    // The server's sentence, capitalized for display.
    expect(toastError).toHaveBeenCalledWith(
      "Only the key's creator or a workspace Admin may change its permissions",
    );
  });

  it("falls back to its own message when the failure has no reason to relay", async () => {
    const mutate = vi.fn().mockRejectedValue(new TypeError("Failed to fetch"));
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useUpdateApiKeyPermissions());
    await act(async () => {
      await result.current.update("key-1", "deploy-agent", GRANTS);
    });

    expect(toastError).toHaveBeenCalledWith(
      "Couldn't update permissions for deploy-agent",
    );
  });

  it("refuses to send until the workspace selection resolves", async () => {
    currentWorkspaceId = null;
    const mutate = vi.fn();
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useUpdateApiKeyPermissions());
    let ok;
    await act(async () => {
      ok = await result.current.update("key-1", "deploy-agent", GRANTS);
    });

    expect(mutate).not.toHaveBeenCalled();
    expect(ok).toBe(false);
  });
});
