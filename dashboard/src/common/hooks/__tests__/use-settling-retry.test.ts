import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { codedGraphQLError } from "@/test/mocks/apollo";
import {
  isBillingSettling,
  SETTLING_RETRY_INTERVAL_MS,
  SETTLING_RETRY_WINDOW_MS,
  useSettlingRetry,
} from "@/common/hooks/use-settling-retry";

const settlingError = () =>
  codedGraphQLError("BILLING_SETTLING", { workspaceId: "tea-1" });

describe("useSettlingRetry", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("retries BILLING_SETTLING every 15 s and resolves with the first success", async () => {
    const attempt = vi
      .fn()
      .mockRejectedValueOnce(settlingError())
      .mockRejectedValueOnce(settlingError())
      .mockResolvedValue("deleted");
    const { result } = renderHook(() => useSettlingRetry());
    let outcome: Promise<unknown> = Promise.resolve();
    await act(async () => {
      outcome = result.current.run(attempt);
    });
    expect(attempt).toHaveBeenCalledTimes(1);
    expect(result.current.settling).toBe(true);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(SETTLING_RETRY_INTERVAL_MS - 1);
    });
    expect(attempt).toHaveBeenCalledTimes(1);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1);
    });
    expect(attempt).toHaveBeenCalledTimes(2);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(SETTLING_RETRY_INTERVAL_MS);
    });
    await expect(outcome).resolves.toBe("deleted");
    expect(attempt).toHaveBeenCalledTimes(3);
    expect(result.current.settling).toBe(false);
  });

  it("gives up after 3 minutes with the BILLING_SETTLING error", async () => {
    const attempt = vi.fn().mockRejectedValue(settlingError());
    const { result } = renderHook(() => useSettlingRetry());
    let outcome: Promise<unknown> = Promise.resolve();
    await act(async () => {
      outcome = result.current.run(attempt);
      outcome.catch(() => {});
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(SETTLING_RETRY_WINDOW_MS + 60_000);
    });
    expect(isBillingSettling(await outcome.catch((err: unknown) => err))).toBe(
      true,
    );
    // Attempts at 0, 15 s, … 180 s.
    expect(attempt).toHaveBeenCalledTimes(
      SETTLING_RETRY_WINDOW_MS / SETTLING_RETRY_INTERVAL_MS + 1,
    );
    expect(result.current.settling).toBe(false);
  });

  it("does not retry any other error", async () => {
    const attempt = vi
      .fn()
      .mockRejectedValue(codedGraphQLError("BILLING_BALANCE_DUE", {}));
    const { result } = renderHook(() => useSettlingRetry());
    await act(async () => {
      await expect(result.current.run(attempt)).rejects.toBeTruthy();
    });
    expect(attempt).toHaveBeenCalledTimes(1);
    expect(result.current.settling).toBe(false);
  });

  it("stops retrying when unmounted", async () => {
    const attempt = vi.fn().mockRejectedValue(settlingError());
    const { result, unmount } = renderHook(() => useSettlingRetry());
    let outcome: Promise<unknown> = Promise.resolve();
    await act(async () => {
      outcome = result.current.run(attempt);
      outcome.catch(() => {});
    });
    unmount();
    await expect(outcome).rejects.toBeTruthy();
    await vi.advanceTimersByTimeAsync(SETTLING_RETRY_WINDOW_MS);
    expect(attempt).toHaveBeenCalledTimes(1);
  });
});
