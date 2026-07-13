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

import { useCronJob } from "@/features/services/hooks/use-cron-job";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useCronJob", () => {
  it("fires updateCronJob with id, schedule, and command; toasts success", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCronJob());
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.updateCronJob("nightly", "0 6 * * *", "node daily.js");
    });

    expect(ok).toBe(true);
    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "nightly", schedule: "0 6 * * *", command: "node daily.js" },
    });
    expect(toastSuccess).toHaveBeenCalled();
    expect(toastError).not.toHaveBeenCalled();
  });

  it("converts an empty command to null in the mutation variables", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCronJob());
    await act(async () => {
      await result.current.updateCronJob("nightly", "0 6 * * *", "");
    });

    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "nightly", schedule: "0 6 * * *", command: null },
    });
  });

  it("toasts an error and resolves false when the mutation rejects", async () => {
    const mutate = vi.fn().mockRejectedValue(new Error("forbidden"));
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCronJob());
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.updateCronJob("nightly", "0 6 * * *", "");
    });

    expect(ok).toBe(false);
    expect(toastError).toHaveBeenCalled();
    expect(toastSuccess).not.toHaveBeenCalled();
  });

  it("tracks busy only for the duration of the in-flight mutation", async () => {
    let resolve: (v: unknown) => void = () => {};
    const mutate = vi.fn(() => new Promise((r) => { resolve = r; }));
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCronJob());
    expect(result.current.busy).toBe(false);

    let pending!: Promise<boolean>;
    act(() => {
      pending = result.current.updateCronJob("nightly", "0 6 * * *", "");
    });
    expect(result.current.busy).toBe(true);

    await act(async () => {
      resolve({});
      await pending;
    });
    expect(result.current.busy).toBe(false);
  });
});
