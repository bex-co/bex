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

import { useDeleteRegistryCredential } from "@/features/registry-credentials/hooks/use-delete-registry-credential";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useDeleteRegistryCredential", () => {
  it("fires deleteRegistryCredential and toasts success", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useDeleteRegistryCredential());
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.remove("rgc-1", "ghcr.io");
    });

    expect(ok).toBe(true);
    expect(mutate).toHaveBeenCalledWith({ variables: { id: "rgc-1" } });
    expect(toastSuccess).toHaveBeenCalledWith("Deleted ghcr.io");
  });

  it("surfaces a mutation failure as an error toast, resolves false, leaves the row listed", async () => {
    const mutate = vi.fn().mockRejectedValue(new Error("forbidden"));
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useDeleteRegistryCredential());
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.remove("rgc-1", "ghcr.io");
    });

    expect(ok).toBe(false);
    expect(toastError).toHaveBeenCalledWith("Couldn't delete ghcr.io");
  });

  it("tracks the deleting id only for the duration of the in-flight mutation", async () => {
    let resolveMutate: (v: unknown) => void = () => {};
    const mutate = vi.fn(
      () =>
        new Promise((resolve) => {
          resolveMutate = resolve;
        }),
    );
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useDeleteRegistryCredential());
    expect(result.current.deleting).toBeNull();

    let pending!: Promise<boolean>;
    act(() => {
      pending = result.current.remove("rgc-1", "ghcr.io");
    });
    expect(result.current.deleting).toBe("rgc-1");

    await act(async () => {
      resolveMutate({});
      await pending;
    });
    expect(result.current.deleting).toBeNull();
  });
});
