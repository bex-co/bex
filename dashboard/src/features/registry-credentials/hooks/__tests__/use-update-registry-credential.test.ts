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

vi.mock("@/common/hooks/use-translations", () => ({
  useTranslations: () => ({ t: (k: string) => k }),
}));

import { useUpdateRegistryCredential } from "@/features/registry-credentials/hooks/use-update-registry-credential";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useUpdateRegistryCredential", () => {
  it("keeps the stored token when authToken is blank — sends null, never a value", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useUpdateRegistryCredential());
    await act(async () => {
      await result.current.update({
        id: "rgc-1",
        name: "New",
        username: "bob",
      });
    });

    expect(mutate).toHaveBeenCalledWith({
      variables: {
        id: "rgc-1",
        name: "New",
        username: "bob",
        authToken: null,
        expiresAt: null,
      },
    });
    expect(toastSuccess).toHaveBeenCalled();
  });

  it("rotates the token when a (trimmed) value is provided", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useUpdateRegistryCredential());
    await act(async () => {
      await result.current.update({ id: "rgc-1", authToken: "  ghp_new  " });
    });

    expect(mutate).toHaveBeenCalledWith({
      variables: {
        id: "rgc-1",
        name: null,
        username: null,
        authToken: "ghp_new",
        expiresAt: null,
      },
    });
  });

  it("surfaces a failed update as an error toast and returns false", async () => {
    const mutate = vi.fn().mockRejectedValue(new Error("nope"));
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useUpdateRegistryCredential());
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.update({
        id: "rgc-1",
        name: "x",
        username: "y",
      });
    });

    expect(ok).toBe(false);
    expect(toastError).toHaveBeenCalled();
    expect(toastSuccess).not.toHaveBeenCalled();
  });
});
