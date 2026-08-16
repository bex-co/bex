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

import { useChangeWorkspacePlan } from "@/features/workspaces/hooks/use-change-workspace-plan";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useChangeWorkspacePlan", () => {
  it("resolves true and toasts success on a successful plan change", async () => {
    const mutate = vi.fn().mockResolvedValue({
      data: {
        changeWorkspacePlan: {
          id: "tea-1",
          name: "acme",
          plan: "pro",
          role: "admin",
          createdAt: null,
        },
      },
    });
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useChangeWorkspacePlan());
    let ok;
    await act(async () => {
      ok = await result.current.changePlan("tea-1", "pro");
    });

    expect(ok).toBe(true);
    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "tea-1", plan: "pro" },
    });
    expect(toastSuccess).toHaveBeenCalledWith("Changed plan to pro");
  });

  it("resolves false and surfaces the backend's downgrade-guard message inline", async () => {
    const mutate = vi
      .fn()
      .mockRejectedValue(
        new Error("workspace has 2 members, exceeds hobby plan's limit of 1"),
      );
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useChangeWorkspacePlan());
    let ok;
    await act(async () => {
      ok = await result.current.changePlan("tea-1", "hobby");
    });

    expect(ok).toBe(false);
    expect(result.current.error).toBe(
      "workspace has 2 members, exceeds hobby plan's limit of 1",
    );
  });
});
