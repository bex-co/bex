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

import { useDisplayName } from "@/features/services/hooks/use-display-name";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useDisplayName", () => {
  it("calls setDisplayName with the immutable id and mutable label", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockReturnValue([mutate]);
    const { result } = renderHook(() => useDisplayName());

    await act(async () => {
      expect(
        await result.current.setDisplayName("stable-id", "Customer API"),
      ).toBe(true);
    });

    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "stable-id", displayName: "Customer API" },
    });
    expect(toastSuccess).toHaveBeenCalledWith(
      'Service renamed to "Customer API".',
    );
  });

  it("reports a forbidden rename and resolves false", async () => {
    mockUseMutation.mockReturnValue([
      vi.fn().mockRejectedValue(new Error("forbidden")),
    ]);
    const { result } = renderHook(() => useDisplayName());

    await act(async () => {
      expect(await result.current.setDisplayName("stable-id", "Nope")).toBe(
        false,
      );
    });

    expect(toastError).toHaveBeenCalledWith(
      "Couldn't rename the service. Please try again.",
    );
  });
});
