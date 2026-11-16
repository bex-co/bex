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

// The minted key binds to the switcher's selection (w6/m18), never a
// workspace the hook resolves itself — same seam useCreateService uses.
let currentWorkspaceId: string | null = "tea-1";
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId }),
}));

import { useCreateApiKey } from "@/features/api-keys/hooks/use-create-api-key";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
  currentWorkspaceId = "tea-1";
});

describe("useCreateApiKey", () => {
  it("uses fetchPolicy no-cache — the key and secret must never enter Apollo's cache", () => {
    mockUseMutation.mockReturnValue([vi.fn()]);
    renderHook(() => useCreateApiKey());
    expect(mockUseMutation).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ fetchPolicy: "no-cache" }),
    );
  });

  it("resolves the minted key (bearer string and secret) and toasts success", async () => {
    // The server's key is carried as issued: this one could not be rebuilt
    // from the id and the secret.
    const mutate = vi.fn().mockResolvedValue({
      data: {
        createApiKey: {
          id: "key-1",
          name: "deploy-agent",
          key: "bex_issued-by-the-server",
          secret: "s3cret",
          createdAt: null,
        },
      },
    });
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCreateApiKey());
    let key;
    await act(async () => {
      key = await result.current.create("deploy-agent");
    });

    expect(key).toEqual({
      id: "key-1",
      name: "deploy-agent",
      key: "bex_issued-by-the-server",
      secret: "s3cret",
    });
    expect(mutate).toHaveBeenCalledWith({
      variables: { name: "deploy-agent", ownerId: "tea-1" },
    });
    expect(toastSuccess).toHaveBeenCalledWith("Created deploy-agent");
  });

  it("composes the key from id and secret when the server answers without one (w7/m426 rollout)", async () => {
    const mutate = vi.fn().mockResolvedValue({
      data: {
        createApiKey: {
          id: "key-1",
          name: "deploy-agent",
          key: null,
          secret: "s3cret",
          createdAt: null,
        },
      },
    });
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCreateApiKey());
    let key;
    await act(async () => {
      key = await result.current.create("deploy-agent");
    });

    expect(key).toEqual({
      id: "key-1",
      name: "deploy-agent",
      key: ["bex", "key-1", "s3cret"].join("_"),
      secret: "s3cret",
    });
    expect(toastSuccess).toHaveBeenCalledWith("Created deploy-agent");
  });

  it("refuses to create until the workspace selection resolves", async () => {
    currentWorkspaceId = null;
    const mutate = vi.fn();
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCreateApiKey());
    let key;
    await act(async () => {
      key = await result.current.create("deploy-agent");
    });

    // Sending a null ownerId would silently mint in the caller's default
    // workspace — the very bug this wiring exists to prevent.
    expect(mutate).not.toHaveBeenCalled();
    expect(key).toBeNull();
    expect(toastError).toHaveBeenCalledWith("Couldn't create deploy-agent");
  });

  it("surfaces a mutation error as a toast and resolves null (t006)", async () => {
    const mutate = vi.fn().mockRejectedValue(new Error("forbidden"));
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCreateApiKey());
    let key;
    await act(async () => {
      key = await result.current.create("deploy-agent");
    });

    expect(key).toBeNull();
    expect(toastError).toHaveBeenCalledWith("Couldn't create deploy-agent");
  });

  it("treats a response with no secret as a failure (server contract violated)", async () => {
    const mutate = vi.fn().mockResolvedValue({
      data: {
        createApiKey: {
          id: "key-1",
          name: "x",
          key: null,
          secret: null,
          createdAt: null,
        },
      },
    });
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCreateApiKey());
    let key;
    await act(async () => {
      key = await result.current.create("x");
    });

    expect(key).toBeNull();
    expect(toastError).toHaveBeenCalled();
  });
});
