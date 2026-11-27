import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { useDisconnectBlueprint } from "@/features/blueprints/hooks/use-disconnect-blueprint";

const mutate = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: () => [mutate],
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
  it("answers null once disconnected", async () => {
    mutate.mockResolvedValue({ data: { disconnectBlueprint: true } });
    const { result } = renderHook(() => useDisconnectBlueprint());

    let refusal: string | null = "unset";
    await act(async () => {
      refusal = await result.current.disconnect("blp-1");
    });

    expect(refusal).toBeNull();
    expect(mutate).toHaveBeenCalledWith({ variables: { id: "blp-1" } });
    expect(toastSuccess).toHaveBeenCalledWith("Blueprint disconnected");
  });

  // Render's Disconnect Blueprint modal shows the refusal in place (w3/m679).
  it("answers the retry sentence on BLUEPRINT_SYNC_BUSY, for the modal to show", async () => {
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

    let refusal: string | null = null;
    await act(async () => {
      refusal = await result.current.disconnect("blp-1");
    });

    expect(refusal).toBe(
      "A sync is currently applying — retry disconnect after it settles",
    );
    expect(toastError).not.toHaveBeenCalled();
  });

  it("answers the server's reason, else the generic failure", async () => {
    const { result } = renderHook(() => useDisconnectBlueprint());
    const refusals: (string | null)[] = [];

    mutate.mockRejectedValueOnce(
      new CombinedGraphQLErrors({
        data: null,
        errors: [
          {
            message: "forbidden: missing can_create",
            extensions: { code: "FORBIDDEN" },
          },
        ],
      }),
    );
    mutate.mockRejectedValueOnce(new Error("Failed to fetch"));
    await act(async () => {
      refusals.push(await result.current.disconnect("blp-1"));
      refusals.push(await result.current.disconnect("blp-1"));
    });

    expect(refusals).toEqual([
      "Forbidden: missing can_create",
      "Disconnect failed",
    ]);
    expect(toastError).not.toHaveBeenCalled();
  });
});
