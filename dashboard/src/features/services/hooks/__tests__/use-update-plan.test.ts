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

import {
  useUpdatePlan,
  type SavedPlan,
} from "@/features/services/hooks/use-update-plan";

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
    let saved: SavedPlan | null | undefined;
    await act(async () => {
      saved = await result.current.updatePlan("app", "pro_plus", "Pro Plus");
    });

    expect(saved).toEqual({ deployId: null });
    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "app", plan: "pro_plus" },
    });
    expect(toastSuccess).toHaveBeenCalledWith(
      "Compute plan updated to Pro Plus",
    );
    expect(toastError).not.toHaveBeenCalled();
  });

  // w3/105: a cron job's plan page opens the deploy the change started.
  it("resolves the deploy the change opened", async () => {
    const mutate = vi.fn().mockResolvedValue({
      data: {
        updateServicePlan: {
          id: "app",
          plan: "pro_plus",
          latestDeployId: "dep-1",
        },
      },
    });
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useUpdatePlan());
    let saved: SavedPlan | null | undefined;
    await act(async () => {
      saved = await result.current.updatePlan("app", "pro_plus", "Pro Plus");
    });

    expect(saved).toEqual({ deployId: "dep-1" });
  });

  it("surfaces a mutation failure as an error toast and resolves null", async () => {
    const mutate = vi.fn().mockRejectedValue(new Error("forbidden"));
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useUpdatePlan());
    let saved: SavedPlan | null | undefined;
    await act(async () => {
      saved = await result.current.updatePlan("app", "pro_plus", "Pro Plus");
    });

    expect(saved).toBeNull();
    expect(toastError).toHaveBeenCalledWith(
      "Couldn't update the compute plan. Please try again.",
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

    let pending!: Promise<SavedPlan | null>;
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
