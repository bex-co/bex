import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { CombinedGraphQLErrors } from "@apollo/client/errors";

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

import { useRuntime } from "@/features/services/hooks/use-runtime";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useRuntime", () => {
  it("sends setRuntime with the chosen runtime and reports success", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockReturnValue([mutate]);
    const { result } = renderHook(() => useRuntime());

    await act(async () => {
      expect(await result.current.setRuntime("srv-web", "python")).toBe(true);
    });

    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "srv-web", runtime: "python" },
    });
    expect(toastSuccess).toHaveBeenCalledWith(
      "Runtime updated. A new deploy has started.",
    );
  });

  it("shows bex-api's refusal as an error instead of a false success", async () => {
    mockUseMutation.mockReturnValue([
      vi.fn().mockRejectedValue(
        new CombinedGraphQLErrors({
          errors: [
            {
              message:
                "bad request: native runtime python requires buildCommand and startCommand",
            },
          ],
        } as never),
      ),
    ]);
    const { result } = renderHook(() => useRuntime());

    await act(async () => {
      expect(await result.current.setRuntime("srv-web", "python")).toBe(false);
    });

    expect(toastSuccess).not.toHaveBeenCalled();
    expect(toastError).toHaveBeenCalledWith(
      "Native runtime python requires buildCommand and startCommand",
    );
  });
});
