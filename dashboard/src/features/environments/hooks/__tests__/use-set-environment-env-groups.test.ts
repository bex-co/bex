import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";

const mutate = vi.fn();
const mockUseMutation = vi.fn(() => [mutate]);
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

import { useSetEnvironmentEnvGroups } from "@/features/environments/hooks/use-set-environment-env-groups";

beforeEach(() => {
  mutate.mockReset();
  mockUseMutation.mockClear();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useSetEnvironmentEnvGroups", () => {
  it("full-replaces ids and refreshes both affected lists", async () => {
    mutate.mockResolvedValue({ data: {} });
    const { result } = renderHook(() => useSetEnvironmentEnvGroups());

    let ok;
    await act(async () => {
      ok = await result.current.setEnvGroups("env-1", "production", [
        "evg-a",
        "evg-b",
      ]);
    });

    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "env-1", envGroupIds: ["evg-a", "evg-b"] },
    });
    expect(mockUseMutation.mock.calls[0][1]).toMatchObject({
      refetchQueries: ["Environments", "EnvGroups"],
      awaitRefetchQueries: true,
    });
    expect(ok).toBe(true);
    expect(toastSuccess).toHaveBeenCalledWith(
      'Environment groups for "production" updated.',
    );
  });

  it("returns false and toasts when replacement fails", async () => {
    mutate.mockRejectedValue(new Error("forbidden"));
    const { result } = renderHook(() => useSetEnvironmentEnvGroups());

    let ok;
    await act(async () => {
      ok = await result.current.setEnvGroups("env-1", "production", []);
    });

    expect(ok).toBe(false);
    expect(toastError).toHaveBeenCalledWith(
      'Failed to update environment groups for "production".',
    );
  });
});
