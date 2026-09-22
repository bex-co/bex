import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { CombinedGraphQLErrors } from "@apollo/client/errors";

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

const ask = vi.fn();
vi.mock("@/common/providers/protected-retry-context", () => ({
  useAskForProtectedConfirmation: () => ask,
}));

import { useCronJob } from "@/features/services/hooks/use-cron-job";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
  ask.mockReset();
});

describe("useCronJob", () => {
  it("fires updateCronJob with id, schedule, and command; toasts success", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCronJob());
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.updateCronJob(
        "nightly",
        "0 6 * * *",
        "node daily.js",
      );
    });

    expect(ok).toBe(true);
    expect(mutate).toHaveBeenCalledWith({
      variables: {
        id: "nightly",
        schedule: "0 6 * * *",
        command: "node daily.js",
      },
    });
    expect(toastSuccess).toHaveBeenCalled();
    expect(toastError).not.toHaveBeenCalled();
  });

  // w4/137: this used to assert the defect. `command || null` collapsed an
  // explicit clear into "keep the existing command", so emptying the field
  // saved successfully, preserved the old command, and toasted success — the
  // backend's nil-means-keep / empty-means-clear contract was never reached.
  it('sends an explicit empty command through as "", which clears the override', async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCronJob());
    await act(async () => {
      await result.current.updateCronJob("nightly", "0 6 * * *", "");
    });

    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "nightly", schedule: "0 6 * * *", command: "" },
    });
  });

  it("still sends null when the caller means keep-the-existing-command", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCronJob());
    await act(async () => {
      await result.current.updateCronJob("nightly", "0 6 * * *", null);
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

  // w1/m145: bex-api's refusal is the only thing that says what to fix, and no
  // retry saves a schedule the server refuses.
  it("toasts bex-api's own reason when the server refuses the update", async () => {
    const mutate = vi.fn().mockRejectedValue(
      new CombinedGraphQLErrors({
        data: null,
        errors: [
          {
            message:
              "bad request: schedule must be a valid 5-field cron expression (e.g. '0 * * * *')",
          },
        ],
      }),
    );
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCronJob());
    await act(async () => {
      await result.current.updateCronJob("nightly", "0 0 * * 7", "");
    });

    expect(toastError).toHaveBeenCalledWith(
      "Schedule must be a valid 5-field cron expression (e.g. '0 * * * *')",
    );
  });

  it("keeps the generic copy when the request never got an answer", async () => {
    const mutate = vi.fn().mockRejectedValue(new TypeError("Failed to fetch"));
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useCronJob());
    await act(async () => {
      await result.current.updateCronJob("nightly", "0 6 * * *", "");
    });

    expect(toastError).toHaveBeenCalledWith(
      "Couldn't save cron job settings. Please try again.",
    );
  });

  it("tracks busy only for the duration of the in-flight mutation", async () => {
    let resolve: (v: unknown) => void = () => {};
    const mutate = vi.fn(
      () =>
        new Promise((r) => {
          resolve = r;
        }),
    );
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

// w4/137: a protected cron's clear must survive the confirmation round trip.
// The retry re-sends the SAME variables with only `confirm` added, so if the
// clear intent were lost anywhere it would be lost here too — the retry is the
// call that actually persists.
describe("useCronJob protected retry preserves an explicit clear", () => {
  const REFUSAL = new Error(
    '"nightly" is a member of a protected environment; retry with confirm="sudo repoint service nightly" to repoint it',
  );

  it("retries the empty command with only the confirmation added", async () => {
    const mutate = vi
      .fn()
      .mockRejectedValueOnce(REFUSAL)
      .mockResolvedValueOnce({});
    mockUseMutation.mockReturnValue([mutate]);
    ask.mockResolvedValue("sudo repoint service nightly");

    const { result } = renderHook(() => useCronJob());
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.updateCronJob("nightly", "0 6 * * *", "");
    });

    expect(ok).toBe(true);
    expect(mutate).toHaveBeenCalledTimes(2);
    expect(mutate.mock.calls[0]?.[0]).toEqual({
      variables: {
        id: "nightly",
        schedule: "0 6 * * *",
        command: "",
        confirm: undefined,
      },
    });
    expect(mutate.mock.calls[1]?.[0]).toEqual({
      variables: {
        id: "nightly",
        schedule: "0 6 * * *",
        command: "",
        confirm: "sudo repoint service nightly",
      },
    });
  });

  it("reports failure without a success toast when the dialog is dismissed", async () => {
    const mutate = vi.fn().mockRejectedValue(REFUSAL);
    mockUseMutation.mockReturnValue([mutate]);
    ask.mockResolvedValue(null);

    const { result } = renderHook(() => useCronJob());
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.updateCronJob("nightly", "0 6 * * *", "");
    });

    expect(ok).toBe(false);
    expect(toastSuccess).not.toHaveBeenCalled();
    expect(toastError).not.toHaveBeenCalled();
  });
});
