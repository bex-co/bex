import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { useSyncBlueprint } from "@/features/blueprints/hooks/use-sync-blueprint";

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

describe("useSyncBlueprint", () => {
  it("returns the protected override phrase without showing a generic error", async () => {
    mutate.mockRejectedValue(
      new Error(
        'service is in a protected environment; retry with confirm="sudo deploy service api"',
      ),
    );
    const { result } = renderHook(() => useSyncBlueprint());

    let outcome;
    await act(async () => {
      outcome = await result.current.sync("blp-1");
    });

    expect(outcome).toEqual({
      status: "confirmation_required",
      confirmation: "sudo deploy service api",
    });
    expect(toastError).not.toHaveBeenCalled();
  });

  it("shows the retry toast on BLUEPRINT_SYNC_BUSY instead of generic failure", async () => {
    mutate.mockRejectedValue(
      new CombinedGraphQLErrors({
        data: null,
        errors: [
          {
            message: "another sync is already running",
            extensions: { code: "BLUEPRINT_SYNC_BUSY" },
          },
        ],
      }),
    );
    const { result } = renderHook(() => useSyncBlueprint());

    let outcome;
    await act(async () => {
      outcome = await result.current.sync("blp-1");
    });

    expect(outcome).toEqual({ status: "error" });
    expect(toastError).toHaveBeenCalledWith(
      "Another sync is already running — retry after it settles",
    );
  });

  it("shows generic failure for non-busy errors", async () => {
    mutate.mockRejectedValue(new Error("boom"));
    const { result } = renderHook(() => useSyncBlueprint());

    await act(async () => {
      await result.current.sync("blp-1");
    });

    expect(toastError).toHaveBeenCalledWith("Sync failed");
  });

  it("forwards the exact phrase on retry and reports success", async () => {
    mutate.mockResolvedValue({
      data: { syncBlueprint: { blueprint: { id: "blp-1" } } },
    });
    const { result } = renderHook(() => useSyncBlueprint());

    let outcome;
    await act(async () => {
      outcome = await result.current.sync("blp-1", "sudo deploy service api");
    });

    expect(mutate).toHaveBeenCalledWith({
      variables: {
        id: "blp-1",
        ownerId: "tea-1",
        confirm: "sudo deploy service api",
      },
    });
    expect(outcome).toMatchObject({ status: "success" });
    expect(toastSuccess).toHaveBeenCalledOnce();
  });
});
