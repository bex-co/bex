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

const reviewed = {
  commitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  path: "render.yaml",
  repo: "https://github.com/a/app",
};

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
      outcome = await result.current.sync("blp-1", { reviewed });
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
      outcome = await result.current.sync("blp-1", { reviewed });
    });

    expect(outcome).toEqual({ status: "error" });
    expect(toastError).toHaveBeenCalledWith(
      "This sync no longer owns execution (busy, interrupted, or superseded) — start a new sync; partial work is never replayed",
    );
  });

  it("surfaces BLUEPRINT_SOURCE_CHANGED for renewed review", async () => {
    mutate.mockRejectedValue(
      new CombinedGraphQLErrors({
        data: null,
        errors: [
          {
            message: "blueprint path no longer matches",
            extensions: { code: "BLUEPRINT_SOURCE_CHANGED" },
          },
        ],
      }),
    );
    const { result } = renderHook(() => useSyncBlueprint());

    let outcome;
    await act(async () => {
      outcome = await result.current.sync("blp-1", { reviewed });
    });

    expect(outcome).toEqual({ status: "source_changed" });
    expect(toastError).toHaveBeenCalledWith(
      "The Blueprint source changed since your review — refresh the preview and confirm again",
    );
  });

  it("shows generic failure for non-busy errors", async () => {
    mutate.mockRejectedValue(new Error("boom"));
    const { result } = renderHook(() => useSyncBlueprint());

    await act(async () => {
      await result.current.sync("blp-1", { reviewed });
    });

    expect(toastError).toHaveBeenCalledWith("Sync failed");
  });

  it("forwards the reviewed source and phrase on retry", async () => {
    mutate.mockResolvedValue({
      data: { syncBlueprint: { blueprint: { id: "blp-1" } } },
    });
    const { result } = renderHook(() => useSyncBlueprint());

    let outcome;
    await act(async () => {
      outcome = await result.current.sync("blp-1", {
        reviewed,
        confirmation: "sudo deploy service api",
      });
    });

    expect(mutate).toHaveBeenCalledWith({
      variables: {
        id: "blp-1",
        ownerId: "tea-1",
        confirm: "sudo deploy service api",
        commitId: reviewed.commitId,
        path: reviewed.path,
        repo: reviewed.repo,
      },
    });
    expect(outcome).toMatchObject({ status: "success" });
    expect(toastSuccess).toHaveBeenCalledOnce();
  });
});
