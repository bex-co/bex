import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mockUseMutation = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: (...args: unknown[]) => mockUseMutation(...args),
}));

const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: {
    success: (...args: unknown[]) => toastSuccess(...args),
    error: (...args: unknown[]) => toastError(...args),
  },
}));

import { useRegistryCredential } from "@/features/services/hooks/use-registry-credential";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useRegistryCredential", () => {
  it("sends the exact selected credential id", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockReturnValue([mutate]);
    const { result } = renderHook(() => useRegistryCredential());

    await act(async () => {
      expect(
        await result.current.setRegistryCredential("srv-web", "rgc-private"),
      ).toBe(true);
    });

    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "srv-web", registryCredentialId: "rgc-private" },
    });
    expect(toastSuccess).toHaveBeenCalledWith("Registry credential updated.");
  });

  it("preserves an explicit empty string for the clear operation", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockReturnValue([mutate]);
    const { result } = renderHook(() => useRegistryCredential());

    await act(async () => {
      expect(await result.current.setRegistryCredential("srv-web", "")).toBe(
        true,
      );
    });

    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "srv-web", registryCredentialId: "" },
    });
    expect(toastSuccess).toHaveBeenCalledWith("Registry credential cleared.");
  });

  it("reports a failed update without claiming success", async () => {
    mockUseMutation.mockReturnValue([
      vi.fn().mockRejectedValue(new Error("forbidden")),
    ]);
    const { result } = renderHook(() => useRegistryCredential());

    await act(async () => {
      expect(
        await result.current.setRegistryCredential("srv-web", "rgc-private"),
      ).toBe(false);
    });

    expect(toastError).toHaveBeenCalledWith(
      "Couldn't update the registry credential.",
    );
  });
});
