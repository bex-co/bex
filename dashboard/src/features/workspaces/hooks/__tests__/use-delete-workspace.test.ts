import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
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

import { codedGraphQLError, uncodedGraphQLError } from "@/test/mocks/apollo";
import { useDeleteWorkspace } from "@/features/workspaces/hooks/use-delete-workspace";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useDeleteWorkspace", () => {
  it("resolves true and toasts success once the confirmation matches", async () => {
    const mutate = vi
      .fn()
      .mockResolvedValue({ data: { deleteWorkspace: "tea-1" } });
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useDeleteWorkspace());
    let ok;
    await act(async () => {
      ok = await result.current.remove(
        "tea-1",
        "acme-hq",
        "sudo delete workspace acme-hq",
      );
    });

    expect(ok).toBe(true);
    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "tea-1", confirmation: "sudo delete workspace acme-hq" },
    });
    expect(toastSuccess).toHaveBeenCalledWith("Deleted acme-hq");
  });

  it("surfaces a mismatched-confirmation refusal inline", async () => {
    const mutate = vi
      .fn()
      .mockRejectedValue(
        uncodedGraphQLError(
          'bad request: confirmation must be "sudo delete workspace acme-hq"',
        ),
      );
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useDeleteWorkspace());
    let ok;
    await act(async () => {
      ok = await result.current.remove("tea-1", "acme-hq", "acme-hq");
    });

    expect(ok).toBe(false);
    expect(result.current.error).toBe(
      'Confirmation must be "sudo delete workspace acme-hq"',
    );
  });

  it("reports BILLING_BALANCE_DUE as a balance with its pay link, not an error", async () => {
    const mutate = vi.fn().mockRejectedValue(
      codedGraphQLError("BILLING_BALANCE_DUE", {
        workspaceId: "tea-1",
        amountDueCents: 300,
        currency: "usd",
        hostedInvoiceUrl: "https://invoice.stripe.com/i/acct_1/in_1",
      }),
    );
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useDeleteWorkspace());
    let ok;
    await act(async () => {
      ok = await result.current.remove(
        "tea-1",
        "acme-hq",
        "sudo delete workspace acme-hq",
      );
    });

    expect(ok).toBe(false);
    expect(result.current.error).toBeNull();
    expect(result.current.balanceDue).toEqual({
      workspaceId: "tea-1",
      amountDueCents: 300,
      currency: "usd",
      hostedInvoiceUrl: "https://invoice.stripe.com/i/acct_1/in_1",
    });
  });

  describe("BILLING_SETTLING", () => {
    beforeEach(() => vi.useFakeTimers());
    afterEach(() => vi.useRealTimers());

    it("shows settling and retries every 15 s until the delete goes through", async () => {
      const mutate = vi
        .fn()
        .mockRejectedValueOnce(codedGraphQLError("BILLING_SETTLING"))
        .mockResolvedValue({ data: { deleteWorkspace: "tea-1" } });
      mockUseMutation.mockReturnValue([mutate]);

      const { result } = renderHook(() => useDeleteWorkspace());
      let ok: Promise<boolean> = Promise.resolve(false);
      await act(async () => {
        ok = result.current.remove(
          "tea-1",
          "acme-hq",
          "sudo delete workspace acme-hq",
        );
      });
      expect(result.current.settling).toBe(true);
      expect(result.current.busy).toBe(true);
      expect(result.current.error).toBeNull();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(15_000);
      });
      expect(await ok).toBe(true);
      expect(mutate).toHaveBeenCalledTimes(2);
      expect(result.current.settling).toBe(false);
      expect(toastSuccess).toHaveBeenCalledWith("Deleted acme-hq");
    });

    it("shows the retry message once 3 minutes of retries run out", async () => {
      const mutate = vi
        .fn()
        .mockRejectedValue(codedGraphQLError("BILLING_SETTLING"));
      mockUseMutation.mockReturnValue([mutate]);

      const { result } = renderHook(() => useDeleteWorkspace());
      let ok: Promise<boolean> = Promise.resolve(true);
      await act(async () => {
        ok = result.current.remove(
          "tea-1",
          "acme-hq",
          "sudo delete workspace acme-hq",
        );
      });
      await act(async () => {
        await vi.advanceTimersByTimeAsync(180_000);
      });
      expect(await ok).toBe(false);
      expect(mutate).toHaveBeenCalledTimes(13);
      expect(result.current.settling).toBe(false);
      expect(result.current.error).toBe(
        "The final usage is still being processed. Try deleting again in a few minutes.",
      );
    });
  });
});
