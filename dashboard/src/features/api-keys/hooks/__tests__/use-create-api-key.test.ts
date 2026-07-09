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

import { useCreateApiKey } from "@/features/api-keys/hooks/use-create-api-key";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useCreateApiKey", () => {
  it("uses fetchPolicy no-cache — the secret must never enter Apollo's cache", () => {
    mockUseMutation.mockReturnValue([vi.fn()]);
    renderHook(() => useCreateApiKey());
    expect(mockUseMutation).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ fetchPolicy: "no-cache" }),
    );
  });

  it("resolves the minted key (with secret) and toasts success", async () => {
    const mutate = vi.fn().mockResolvedValue({
      data: { createApiKey: { id: "key-1", name: "deploy-agent", secret: "s3cret", createdAt: null } },
    });
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCreateApiKey());
    let key;
    await act(async () => {
      key = await result.current.create("deploy-agent");
    });

    expect(key).toEqual({ id: "key-1", name: "deploy-agent", secret: "s3cret" });
    expect(mutate).toHaveBeenCalledWith({ variables: { name: "deploy-agent" } });
    expect(toastSuccess).toHaveBeenCalledWith("Created deploy-agent");
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
      data: { createApiKey: { id: "key-1", name: "x", secret: null, createdAt: null } },
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
