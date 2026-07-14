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

import { usePreDeployCommand } from "@/features/services/hooks/use-pre-deploy-command";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("usePreDeployCommand", () => {
  it("fires setPreDeployCommand with the id and command and toasts success", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => usePreDeployCommand());
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.setPreDeployCommand("app", "npm run migrate");
    });

    expect(ok).toBe(true);
    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "app", command: "npm run migrate" },
    });
    expect(toastSuccess).toHaveBeenCalledWith("Pre-Deploy Command updated.");
    expect(toastError).not.toHaveBeenCalled();
  });

  it("allows an empty string (clearing the pre-deploy step)", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => usePreDeployCommand());
    await act(async () => {
      await result.current.setPreDeployCommand("app", "");
    });

    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "app", command: "" },
    });
  });

  it("surfaces a mutation failure as an error toast and resolves false", async () => {
    const mutate = vi.fn().mockRejectedValue(new Error("forbidden"));
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => usePreDeployCommand());
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.setPreDeployCommand("app", "npm run migrate");
    });

    expect(ok).toBe(false);
    expect(toastError).toHaveBeenCalledWith(
      "Couldn't update the Pre-Deploy Command. Please try again.",
    );
    expect(toastSuccess).not.toHaveBeenCalled();
  });
});
