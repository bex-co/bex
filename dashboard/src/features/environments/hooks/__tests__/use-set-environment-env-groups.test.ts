import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";

const mutate = vi.fn();
const mockUseMutation = vi.fn((..._args: unknown[]) => [mutate]);
const refetchQueries = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: (...args: unknown[]) => mockUseMutation(...args),
  useApolloClient: () => ({ refetchQueries }),
}));

const toastSuccess = vi.fn();
const toastError = vi.fn();
const toastWarning = vi.fn();
vi.mock("sonner", () => ({
  toast: {
    success: (...args: unknown[]) => toastSuccess(...args),
    error: (...args: unknown[]) => toastError(...args),
    warning: (...args: unknown[]) => toastWarning(...args),
  },
}));

import { useSetEnvironmentEnvGroups } from "@/features/environments/hooks/use-set-environment-env-groups";

beforeEach(() => {
  mutate.mockReset();
  mockUseMutation.mockClear();
  toastSuccess.mockReset();
  toastError.mockReset();
  toastWarning.mockReset();
  refetchQueries.mockReset().mockResolvedValue([]);
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

  // w4/m206: a failed replacement may have moved some groups, or all of them
  // when only its acknowledgment was lost. The hook reads placement before it
  // lets go, so the next write starts from where the groups really are.
  it("reads placement after a failure before it reports the failure", async () => {
    mutate.mockRejectedValue(new Error("timed out"));
    let releaseRead: () => void = () => undefined;
    refetchQueries.mockReturnValue(
      new Promise<void>((resolve) => {
        releaseRead = resolve;
      }),
    );
    const { result } = renderHook(() => useSetEnvironmentEnvGroups());

    let settled: boolean | undefined;
    act(() => {
      void result.current
        .setEnvGroups("env-1", "production", ["evg-a", "evg-b"])
        .then((ok) => {
          settled = ok;
        });
    });
    await vi.waitFor(() => expect(refetchQueries).toHaveBeenCalledTimes(1));
    expect(refetchQueries).toHaveBeenCalledWith({
      include: ["Environments", "EnvGroups"],
    });
    // Still busy: the caller's Move stays unavailable until the read settles.
    expect(settled).toBeUndefined();
    expect(result.current.busyId).toBe("env-1");
    expect(toastError).toHaveBeenCalledTimes(1);

    await act(async () => {
      releaseRead();
    });
    await vi.waitFor(() => expect(settled).toBe(false));
    expect(result.current.busyId).toBeNull();
    expect(mutate).toHaveBeenCalledTimes(1);
  });

  it("offers the read again, never the write, when placement cannot be read", async () => {
    mutate.mockRejectedValue(new Error("timed out"));
    refetchQueries.mockRejectedValueOnce(new Error("offline"));
    const { result } = renderHook(() => useSetEnvironmentEnvGroups());

    await act(async () => {
      await result.current.setEnvGroups("env-1", "production", ["evg-a"]);
    });
    expect(toastWarning).toHaveBeenCalledTimes(1);
    expect(toastWarning.mock.calls[0][0]).toMatch(/Couldn't confirm where/);

    const retry = toastWarning.mock.calls[0][1] as {
      action: { onClick: () => void };
    };
    await act(async () => {
      retry.action.onClick();
    });
    expect(refetchQueries).toHaveBeenCalledTimes(2);
    expect(mutate).toHaveBeenCalledTimes(1);
    expect(toastWarning).toHaveBeenCalledTimes(1);
  });

  it("reads nothing extra after a successful replacement", async () => {
    mutate.mockResolvedValue({ data: {} });
    const { result } = renderHook(() => useSetEnvironmentEnvGroups());
    await act(async () => {
      await result.current.setEnvGroups("env-1", "production", ["evg-a"]);
    });
    expect(refetchQueries).not.toHaveBeenCalled();
  });
});
