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

import { useScaleService } from "@/features/services/hooks/use-scale-service";
import enServices from "@/features/services/locales/en";
import zhServices from "@/features/services/locales/zh";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useScaleService", () => {
  it("keeps English and Chinese acknowledgements in asynchronous tense", () => {
    expect(enServices["services.scaleSuccess"].message).toBe(
      "Scaling to {count} instance(s)…",
    );
    expect(zhServices["services.scaleSuccess"].message).toBe(
      "正在缩放至 {count} 个实例…",
    );
  });

  it("acknowledges accepted intent without claiming convergence", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockReturnValue([mutate, { loading: false }]);
    const { result } = renderHook(() => useScaleService());

    await act(async () => {
      expect(await result.current.scaleService("srv-web", 2)).toBe(true);
    });

    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "srv-web", numInstances: 2 },
    });
    expect(toastSuccess).toHaveBeenCalledWith("Scaling to 2 instance(s)…");
    expect(toastSuccess).not.toHaveBeenCalledWith(
      expect.stringMatching(/^Scaled/),
    );
    expect(toastError).not.toHaveBeenCalled();
  });

  it("preserves the mutation rejection as an error and returns false", async () => {
    mockUseMutation.mockReturnValue([
      vi.fn().mockRejectedValue(new Error("forbidden")),
      { loading: false },
    ]);
    const { result } = renderHook(() => useScaleService());

    await act(async () => {
      expect(await result.current.scaleService("srv-web", 2)).toBe(false);
    });

    expect(toastError).toHaveBeenCalledWith("Failed to update instance count.");
    expect(toastSuccess).not.toHaveBeenCalled();
  });
});
