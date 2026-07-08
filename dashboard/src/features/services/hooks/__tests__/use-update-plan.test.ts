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

import { useUpdatePlan } from "@/features/services/hooks/use-update-plan";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useUpdatePlan", () => {
  it("fires updateServicePlan with the picked id and toasts success on the display name", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useUpdatePlan());
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.updatePlan("app", "pro_plus", "Pro Plus");
    });

    expect(ok).toBe(true);
    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "app", plan: "pro_plus" },
    });
    expect(toastSuccess).toHaveBeenCalledWith("Instance type updated to Pro Plus");
    expect(toastError).not.toHaveBeenCalled();
  });

  it("surfaces a mutation failure as an error toast and resolves false", async () => {
    const mutate = vi.fn().mockRejectedValue(new Error("forbidden"));
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useUpdatePlan());
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.updatePlan("app", "pro_plus", "Pro Plus");
    });

    expect(ok).toBe(false);
    expect(toastError).toHaveBeenCalledWith(
      "Couldn't update the instance type. Please try again.",
    );
    expect(toastSuccess).not.toHaveBeenCalled();
  });

  it("tracks busy only for the duration of the in-flight mutation", async () => {
    let resolveMutate: (v: unknown) => void = () => {};
    const mutate = vi.fn(
      () =>
        new Promise((resolve) => {
          resolveMutate = resolve;
        }),
    );
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useUpdatePlan());
    expect(result.current.busy).toBe(false);

    let pending!: Promise<boolean>;
    act(() => {
      pending = result.current.updatePlan("app", "pro_plus", "Pro Plus");
    });
    expect(result.current.busy).toBe(true);

    await act(async () => {
      resolveMutate({});
      await pending;
    });
    expect(result.current.busy).toBe(false);
  });
});
