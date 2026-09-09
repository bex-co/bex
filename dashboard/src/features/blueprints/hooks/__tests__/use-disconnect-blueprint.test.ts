import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { useDisconnectBlueprint } from "@/features/blueprints/hooks/use-disconnect-blueprint";

const mutate = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: () => [mutate],
}));

vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-1" }),
}));

const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: {
    success: (...args: unknown[]) => toastSuccess(...args),
    error: (...args: unknown[]) => toastError(...args),
  },
}));

beforeEach(() => {
  mutate.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useDisconnectBlueprint", () => {
  it("shows the retry toast on BLUEPRINT_SYNC_BUSY instead of generic failure", async () => {
    mutate.mockRejectedValue(
      new CombinedGraphQLErrors({
        data: null,
        errors: [
          {
            message: "a sync is currently applying",
            extensions: { code: "BLUEPRINT_SYNC_BUSY" },
          },
        ],
      }),
    );
    const { result } = renderHook(() => useDisconnectBlueprint());

    let ok;
    await act(async () => {
      ok = await result.current.disconnect("blp-1");
    });

    expect(ok).toBe(false);
    expect(toastError).toHaveBeenCalledWith(
      "A sync is currently applying — retry disconnect after it settles",
    );
  });

  it("shows generic failure for non-busy errors", async () => {
    mutate.mockRejectedValue(new Error("boom"));
    const { result } = renderHook(() => useDisconnectBlueprint());

    await act(async () => {
      await result.current.disconnect("blp-1");
    });

    expect(toastError).toHaveBeenCalledWith("Disconnect failed");
  });
});
