import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useRecovery } from "@/features/databases/hooks/use-recovery";

const useQuery = vi.fn();
const useMutation = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => useQuery(...args),
  useMutation: (...args: unknown[]) => useMutation(...args),
}));

const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: {
    success: (...args: unknown[]) => toastSuccess(...args),
    error: (...args: unknown[]) => toastError(...args),
  },
}));

const createExportMutation = vi.fn();
const recoverMutation = vi.fn();
const stopPolling = vi.fn();

beforeEach(() => {
  vi.clearAllMocks();
  useQuery.mockReturnValue({
    data: undefined,
    loading: false,
    refetch: vi.fn(async () => undefined),
    startPolling: vi.fn(),
    stopPolling,
  });
  useMutation
    .mockReturnValueOnce([createExportMutation, { loading: false }])
    .mockReturnValueOnce([recoverMutation, { loading: false }]);
});

describe("useRecovery post-create identity", () => {
  it("does not poll exports while recovery information is unreadable", () => {
    const infoError = new Error("forbidden");
    const startExportPolling = vi.fn();
    const stopExportPolling = vi.fn();
    useQuery
      .mockReturnValueOnce({
        data: {
          databaseRecoveryInfo: {
            enabled: true,
            earliestRecoveryTime: "2026-07-13T12:00:00Z",
            latestRecoveryTime: "2026-07-14T12:00:00Z",
            backups: [],
          },
        },
        error: infoError,
        loading: false,
      })
      .mockReturnValueOnce({
        data: undefined,
        refetch: vi.fn(async () => undefined),
        startPolling: startExportPolling,
        stopPolling: stopExportPolling,
      });

    const { result } = renderHook(() => useRecovery("dpg-source"));

    expect(useQuery.mock.calls[1]?.[1]).toMatchObject({ skip: true });
    expect(startExportPolling).not.toHaveBeenCalled();
    expect(stopExportPolling).toHaveBeenCalled();
    expect(result.current.error).toBe(infoError);
  });

  it("returns the recovered database id from the mutation", async () => {
    recoverMutation.mockResolvedValue({
      data: { recoverDatabase: { id: "dpg-recovered-id" } },
    });
    const { result } = renderHook(() => useRecovery("dpg-source"));

    let recoveredId: string | null | undefined;
    await act(async () => {
      recoveredId = await result.current.recover({
        name: "recovered",
        targetTime: "2026-07-17T12:00:00Z",
      });
    });

    expect(recoveredId).toBe("dpg-recovered-id");
    expect(recoverMutation).toHaveBeenCalledWith({
      variables: {
        id: "dpg-source",
        name: "recovered",
        targetTime: "2026-07-17T12:00:00Z",
      },
    });
    expect(toastSuccess).toHaveBeenCalledWith("Restoring into recovered…");
  });

  it("returns null instead of inventing an id when recovery omits it", async () => {
    recoverMutation.mockResolvedValue({
      data: { recoverDatabase: { id: null } },
    });
    const { result } = renderHook(() => useRecovery("dpg-source"));

    let recoveredId: string | null | undefined;
    await act(async () => {
      recoveredId = await result.current.recover({ name: "recovered" });
    });

    expect(recoveredId).toBeNull();
    expect(toastSuccess).not.toHaveBeenCalled();
    expect(toastError).toHaveBeenCalledWith(
      "Couldn't restore: recoverDatabase returned no id",
    );
  });
});
