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

import { useFieldMutation } from "@/features/services/hooks/use-field-mutation";
import { SetBranchDocument } from "@/graphql/definitions";

const keys = {
  success: "services.buildDeploySuccess",
  error: "services.buildDeployError",
};

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

// Thirteen single-field Settings hooks delegate their whole body here, and most
// have no direct test of their own — so the contract they all inherit (variables
// mapping, both toasts, the boolean result, and busy always clearing) is pinned
// here rather than thirteen times.
describe("useFieldMutation", () => {
  function setup(mutate: ReturnType<typeof vi.fn>) {
    mockUseMutation.mockReturnValue([mutate]);
    return renderHook(() =>
      useFieldMutation(
        SetBranchDocument,
        (id: string, branch: string) => ({ id, branch }),
        keys,
      ),
    );
  }

  it("maps its arguments through toVariables and toasts success", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    const { result } = setup(mutate);

    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.run("srv-1", "main");
    });

    expect(ok).toBe(true);
    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "srv-1", branch: "main" },
    });
    expect(toastSuccess).toHaveBeenCalledTimes(1);
    expect(toastError).not.toHaveBeenCalled();
  });

  it("resolves false and toasts the error when the mutation rejects", async () => {
    const mutate = vi.fn().mockRejectedValue(new Error("nope"));
    const { result } = setup(mutate);

    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.run("srv-1", "main");
    });

    expect(ok).toBe(false);
    expect(toastError).toHaveBeenCalledTimes(1);
    expect(toastSuccess).not.toHaveBeenCalled();
  });

  // The `finally` is the point: a failed write must not leave the control
  // disabled forever.
  it("clears busy after a rejected mutation", async () => {
    const mutate = vi.fn().mockRejectedValue(new Error("nope"));
    const { result } = setup(mutate);

    await act(async () => {
      await result.current.run("srv-1", "main");
    });

    expect(result.current.busy).toBe(false);
  });
});
