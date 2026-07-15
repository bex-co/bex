import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";

const mockUseMutation = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: (...args: unknown[]) => mockUseMutation(...args),
}));

const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: {
    success: (...args: unknown[]) => toastSuccess(...args),
    error: (...args: unknown[]) => toastError(...args),
  },
}));

import { useMaxShutdownDelay } from "@/features/services/hooks/use-max-shutdown-delay";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useMaxShutdownDelay", () => {
  it("calls setMaxShutdownDelay with the service and seconds", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockReturnValue([mutate]);
    const { result } = renderHook(() => useMaxShutdownDelay());

    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.setMaxShutdownDelay("worker", 120);
    });

    expect(ok).toBe(true);
    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "worker", seconds: 120 },
    });
    expect(toastSuccess).toHaveBeenCalledWith("Max shutdown delay updated.");
  });

  it("returns false and reports a mutation failure", async () => {
    mockUseMutation.mockReturnValue([
      vi.fn().mockRejectedValue(new Error("forbidden")),
    ]);
    const { result } = renderHook(() => useMaxShutdownDelay());

    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.setMaxShutdownDelay("web", 90);
    });

    expect(ok).toBe(false);
    expect(toastError).toHaveBeenCalledWith(
      "Couldn't update the max shutdown delay.",
    );
  });
});
