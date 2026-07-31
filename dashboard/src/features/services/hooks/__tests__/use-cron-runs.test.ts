import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";

const mockUseQuery = vi.fn();
const mockUseMutation = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
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

vi.mock("@/common/hooks/use-translations", () => ({
  useTranslations: () => ({ t: (k: string) => k }),
}));

import { useCronRuns } from "@/features/services/hooks/use-cron-runs";

const refetch = vi.fn();

function mockQuery(cronJobRuns: unknown[]) {
  mockUseQuery.mockReturnValue({
    data: { cronJobRuns },
    loading: false,
    error: undefined,
    fetchMore: vi.fn(),
    refetch,
  });
}

beforeEach(() => {
  mockUseQuery.mockReset();
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
  refetch.mockReset();
  refetch.mockResolvedValue({});
});

describe("useCronRuns trigger (w5/m60)", () => {
  it("fires runCronJob with the service id and refetches the history on success", async () => {
    mockQuery([]);
    const runCronJob = vi.fn().mockResolvedValue({});
    mockUseMutation.mockReturnValue([runCronJob]);

    const { result } = renderHook(() => useCronRuns("nightly"));
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.trigger();
    });

    expect(ok).toBe(true);
    expect(runCronJob).toHaveBeenCalledWith({ variables: { id: "nightly" } });
    expect(refetch).toHaveBeenCalled();
    expect(toastSuccess).toHaveBeenCalled();
    expect(result.current.triggerError).toBeNull();
  });

  it("surfaces the backend's active-run rejection inline (not a toast) and returns false", async () => {
    mockQuery([]);
    const rejection = new Error("a run is already active");
    const runCronJob = vi.fn().mockRejectedValue(rejection);
    mockUseMutation.mockReturnValue([runCronJob]);

    const { result } = renderHook(() => useCronRuns("nightly"));
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.trigger();
    });

    expect(ok).toBe(false);
    expect(result.current.triggerError).toBe("a run is already active");
    expect(refetch).not.toHaveBeenCalled();

    act(() => result.current.clearTriggerError());
    expect(result.current.triggerError).toBeNull();
  });

  it("flags hasActiveRun while a run is pending/running", () => {
    mockQuery([
      { id: "crr-1", status: "pending", startedAt: null, finishedAt: null },
    ]);
    mockUseMutation.mockReturnValue([vi.fn()]);
    const { result } = renderHook(() => useCronRuns("nightly"));
    expect(result.current.hasActiveRun).toBe(true);
  });
});
