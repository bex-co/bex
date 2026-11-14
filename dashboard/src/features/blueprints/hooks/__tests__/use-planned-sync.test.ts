import { describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { usePlannedSync } from "@/features/blueprints/hooks/use-planned-sync";
import { codedGraphQLError } from "@/test/mocks/apollo";

const approve = vi.fn(async () => ({ data: {} }));
const refetch = vi.fn(async () => ({}));
const toastError = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: () => ({
    refetch,
    loading: false,
    data: {
      blueprintPlannedSync: {
        id: "exe-planned-on-screen",
        commitId: "c1",
        actions: [],
        requiredManualEnvVars: [],
      },
    },
  }),
  useMutation: () => [approve],
}));
vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: (...args: unknown[]) => toastError(...args),
  },
}));
vi.mock("@/features/usage/context/payment-required-context", () => ({
  usePaymentRequiredGate: () => ({ run: (fn: () => unknown) => fn() }),
}));

// w5/m384: approving binds the plan the page shows, so a plan someone else
// made meanwhile is refused by the server rather than applied.
describe("usePlannedSync", () => {
  it("approves with the sync id of the plan on screen", async () => {
    const { result } = renderHook(() => usePlannedSync("exs-1", true));
    await act(async () => {
      await result.current.approve({});
    });
    expect(approve).toHaveBeenCalledWith(
      expect.objectContaining({
        variables: expect.objectContaining({
          id: "exs-1",
          syncId: "exe-planned-on-screen",
        }),
      }),
    );
  });

  it("shows the newest plan when the one on screen was replaced", async () => {
    approve.mockRejectedValueOnce(codedGraphQLError("BLUEPRINT_PLAN_CHANGED"));
    const { result } = renderHook(() => usePlannedSync("exs-1", true));
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.approve({});
    });
    expect(ok).toBe(false);
    expect(refetch).toHaveBeenCalled();
    expect(toastError).toHaveBeenCalledWith(
      expect.stringContaining("Review the newest plan"),
    );
  });
});
