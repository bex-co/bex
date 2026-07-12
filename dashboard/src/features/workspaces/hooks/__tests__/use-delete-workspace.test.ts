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

import { useDeleteWorkspace } from "@/features/workspaces/hooks/use-delete-workspace";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useDeleteWorkspace", () => {
  it("resolves true and toasts success once the confirmation matches", async () => {
    const mutate = vi.fn().mockResolvedValue({ data: { deleteWorkspace: "tea-1" } });
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useDeleteWorkspace());
    let ok;
    await act(async () => {
      ok = await result.current.remove(
        "tea-1",
        "acme-hq",
        "sudo delete workspace acme-hq",
      );
    });

    expect(ok).toBe(true);
    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "tea-1", confirmation: "sudo delete workspace acme-hq" },
    });
    expect(toastSuccess).toHaveBeenCalledWith("Deleted acme-hq");
  });

  it("surfaces a mismatched-confirmation refusal inline", async () => {
    const mutate = vi.fn().mockRejectedValue(
      new Error('bad request: confirmation must be "sudo delete workspace acme-hq"'),
    );
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useDeleteWorkspace());
    let ok;
    await act(async () => {
      ok = await result.current.remove("tea-1", "acme-hq", "acme-hq");
    });

    expect(ok).toBe(false);
    expect(result.current.error).toBe(
      'bad request: confirmation must be "sudo delete workspace acme-hq"',
    );
  });
});
