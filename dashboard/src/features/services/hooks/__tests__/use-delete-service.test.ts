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

import { useDeleteService } from "@/features/services/hooks/use-delete-service";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useDeleteService", () => {
  it("fires deleteService with the id and toasts success on the name", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useDeleteService());
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.remove("web", "web");
    });

    expect(ok).toBe(true);
    expect(mutate).toHaveBeenCalledWith(
      expect.objectContaining({ variables: { id: "web" } }),
    );
    expect(toastSuccess).toHaveBeenCalledWith("Deleted web");
    expect(toastError).not.toHaveBeenCalled();
  });

  it("evicts the deleted Service from the cache so the list drops the row", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useDeleteService());
    await act(async () => {
      await result.current.remove("web", "web");
    });

    // The mutation carries an `update` that evicts the normalized Service entity
    // — that's what removes it from every cached `services` list read.
    const { update } = mutate.mock.calls[0][0];
    const cache = {
      identify: vi.fn().mockReturnValue("Service:web"),
      evict: vi.fn(),
      gc: vi.fn(),
    };
    update(cache);

    expect(cache.identify).toHaveBeenCalledWith({
      __typename: "Service",
      id: "web",
    });
    expect(cache.evict).toHaveBeenCalledWith({ id: "Service:web" });
    expect(cache.gc).toHaveBeenCalled();
  });

  it("surfaces a mutation failure as an error toast and resolves false", async () => {
    const mutate = vi.fn().mockRejectedValue(new Error("forbidden"));
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useDeleteService());
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.remove("web", "web");
    });

    expect(ok).toBe(false);
    expect(toastError).toHaveBeenCalledWith("Couldn't delete web");
    expect(toastSuccess).not.toHaveBeenCalled();
  });

  it("tracks deleting only for the duration of the in-flight mutation", async () => {
    let resolveMutate: (v: unknown) => void = () => {};
    const mutate = vi.fn(
      () =>
        new Promise((resolve) => {
          resolveMutate = resolve;
        }),
    );
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useDeleteService());
    expect(result.current.deleting).toBe(false);

    let pending!: Promise<boolean>;
    act(() => {
      pending = result.current.remove("web", "web");
    });
    expect(result.current.deleting).toBe(true);

    await act(async () => {
      resolveMutate({});
      await pending;
    });
    expect(result.current.deleting).toBe(false);
  });
});
