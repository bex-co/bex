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

import { uncodedGraphQLError } from "@/test/mocks/apollo";
import { useRenameWorkspace } from "@/features/workspaces/hooks/use-rename-workspace";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useRenameWorkspace", () => {
  it("resolves true and toasts success on a successful rename", async () => {
    const mutate = vi.fn().mockResolvedValue({
      data: {
        renameWorkspace: {
          id: "tea-1",
          name: "acme-renamed",
          plan: "hobby",
          role: "admin",
          createdAt: null,
        },
      },
    });
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useRenameWorkspace());
    let ok;
    await act(async () => {
      ok = await result.current.rename("tea-1", "acme-renamed");
    });

    expect(ok).toBe(true);
    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "tea-1", name: "acme-renamed" },
    });
    expect(toastSuccess).toHaveBeenCalledWith("Renamed to acme-renamed");
  });

  it("resolves false and surfaces the backend error inline on a forbidden rename", async () => {
    const mutate = vi
      .fn()
      .mockRejectedValue(
        uncodedGraphQLError("forbidden: you are not an admin of tea-1"),
      );
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useRenameWorkspace());
    let ok;
    await act(async () => {
      ok = await result.current.rename("tea-1", "acme-renamed");
    });

    expect(ok).toBe(false);
    expect(result.current.error).toBe(
      "Forbidden: you are not an admin of tea-1",
    );
  });

  it("shows the translated fallback, not the browser's text, on a network failure", async () => {
    const mutate = vi.fn().mockRejectedValue(new TypeError("Failed to fetch"));
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useRenameWorkspace());
    await act(async () => {
      await result.current.rename("tea-1", "acme-renamed");
    });
    expect(result.current.error).toBe("Couldn't rename the workspace");
  });
});
