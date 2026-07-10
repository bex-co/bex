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

import { useRootDir } from "@/features/services/hooks/use-root-dir";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useRootDir", () => {
  it("fires setRootDir with the id and value and toasts success", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useRootDir());
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.setRootDir("app", "backend");
    });

    expect(ok).toBe(true);
    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "app", rootDir: "backend" },
    });
    expect(toastSuccess).toHaveBeenCalledWith("Root Directory updated.");
    expect(toastError).not.toHaveBeenCalled();
  });

  it("allows an empty string (restoring the repo root)", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useRootDir());
    await act(async () => {
      await result.current.setRootDir("app", "");
    });

    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "app", rootDir: "" },
    });
  });

  it("surfaces a mutation failure as an error toast and resolves false", async () => {
    const mutate = vi.fn().mockRejectedValue(new Error("forbidden"));
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useRootDir());
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.setRootDir("app", "backend");
    });

    expect(ok).toBe(false);
    expect(toastError).toHaveBeenCalledWith(
      "Couldn't update the Root Directory. Please try again.",
    );
    expect(toastSuccess).not.toHaveBeenCalled();
  });

  it("tracks busy only for the duration of the in-flight mutation", async () => {
    let resolveMutate: (v: unknown) => void = () => {};
    const mutate = vi.fn(
      () =>
        new Promise((resolve) => {
          resolveMutate = resolve;
        }),
    );
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useRootDir());
    expect(result.current.busy).toBe(false);

    let pending!: Promise<boolean>;
    act(() => {
      pending = result.current.setRootDir("app", "backend");
    });
    expect(result.current.busy).toBe(true);

    await act(async () => {
      resolveMutate({});
      await pending;
    });
    expect(result.current.busy).toBe(false);
  });
});
