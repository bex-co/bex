import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

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

let currentWorkspaceId: string | null = "tea-1";
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId }),
}));

import { useResendWebhookDelivery } from "@/features/webhooks/hooks/use-resend-webhook-delivery";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
  currentWorkspaceId = "tea-1";
});

describe("useResendWebhookDelivery", () => {
  it("sends owner, attempt, endpoint, and a required idempotency key", async () => {
    const mutate = vi.fn().mockResolvedValue({
      data: { resendWebhookDelivery: { id: "whd-reserved" } },
    });
    mockUseMutation.mockReturnValue([mutate]);
    const { result } = renderHook(() => useResendWebhookDelivery());

    let ok = false;
    await act(async () => {
      ok = await result.current.resend("whk-1", "whd-failed");
    });

    expect(ok).toBe(true);
    expect(mutate).toHaveBeenCalledWith({
      variables: {
        endpointId: "whk-1",
        attemptId: "whd-failed",
        ownerId: "tea-1",
        idempotencyKey: expect.stringMatching(/^dashboard-/),
      },
    });
    expect(toastSuccess).toHaveBeenCalled();
  });

  it("reuses the same key when an ambiguous action is retried", async () => {
    const mutate = vi
      .fn()
      .mockRejectedValueOnce(new Error("connection reset"))
      .mockResolvedValueOnce({
        data: { resendWebhookDelivery: { id: "whd-reserved" } },
      });
    mockUseMutation.mockReturnValue([mutate]);
    const { result } = renderHook(() => useResendWebhookDelivery());

    await act(async () => {
      await result.current.resend("whk-1", "whd-failed");
      await result.current.resend("whk-1", "whd-failed");
    });

    const firstKey = mutate.mock.calls[0][0].variables.idempotencyKey;
    const secondKey = mutate.mock.calls[1][0].variables.idempotencyKey;
    expect(secondKey).toBe(firstKey);
    expect(toastError).toHaveBeenCalledTimes(1);
    expect(toastSuccess).toHaveBeenCalledTimes(1);
  });

  it("does not call the mutation before workspace selection resolves", async () => {
    currentWorkspaceId = null;
    const mutate = vi.fn();
    mockUseMutation.mockReturnValue([mutate]);
    const { result } = renderHook(() => useResendWebhookDelivery());

    let ok = true;
    await act(async () => {
      ok = await result.current.resend("whk-1", "whd-failed");
    });

    expect(ok).toBe(false);
    expect(mutate).not.toHaveBeenCalled();
  });
});
