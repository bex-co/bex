import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import {
  useUpdateBlueprint,
  type BlueprintUpdateResult,
} from "@/features/blueprints/hooks/use-update-blueprint";

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

describe("useUpdateBlueprint", () => {
  it("sends only the row's field, a branch included (w3/m679)", async () => {
    mutate.mockResolvedValue({ data: { updateBlueprint: { id: "exs-1" } } });
    const { result } = renderHook(() => useUpdateBlueprint());

    let outcome: BlueprintUpdateResult | undefined;
    await act(async () => {
      outcome = await result.current.update("exs-1", { branch: "staging" });
    });

    expect(outcome).toEqual({ ok: true });
    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "exs-1", branch: "staging" },
    });
    expect(toastSuccess).toHaveBeenCalledWith("Blueprint updated");
  });

  // Render words a refusal at the row that caused it, so the hook hands the
  // error back instead of toasting it.
  it("returns a refusal for the row to word, and toasts nothing", async () => {
    const refusal = new CombinedGraphQLErrors({
      data: null,
      errors: [
        {
          message: 'Blueprint file "x.yaml" not found on branch "main".',
          extensions: { code: "file_not_found" },
        },
      ],
    });
    mutate.mockRejectedValue(refusal);
    const { result } = renderHook(() => useUpdateBlueprint());

    let outcome: BlueprintUpdateResult | undefined;
    await act(async () => {
      outcome = await result.current.update("exs-1", { path: "x.yaml" });
    });

    expect(outcome).toEqual({ ok: false, error: refusal });
    expect(toastError).not.toHaveBeenCalled();
    expect(toastSuccess).not.toHaveBeenCalled();
    expect(result.current.busy).toBe(false);
  });
});
