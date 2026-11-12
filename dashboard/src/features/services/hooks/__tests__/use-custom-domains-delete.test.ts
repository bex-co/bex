import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { codedGraphQLError } from "@/test/mocks/apollo";
import { useCustomDomainMutations } from "@/features/services/hooks/use-custom-domains";

const refetch = vi.fn();
const deleteMutation = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useMutation: () => [deleteMutation],
}));
const toast = vi.hoisted(() => ({
  success: vi.fn(),
  error: vi.fn(),
  info: vi.fn(),
}));
vi.mock("sonner", () => ({ toast }));

beforeEach(() => {
  refetch.mockReset().mockResolvedValue({});
  deleteMutation.mockReset();
  toast.success.mockReset();
  toast.error.mockReset();
});

// w4/574: bex-api answers NOT_FOUND for a domain already removed, e.g. from
// another tab. The delete has done what was asked, so the list catches up and
// no error is shown; any other refusal still is.
describe("useCustomDomainMutations deleteDomain", () => {
  it("refreshes the list without an error when the domain is already gone", async () => {
    deleteMutation.mockRejectedValue(codedGraphQLError("NOT_FOUND"));
    const { result } = renderHook(() =>
      useCustomDomainMutations("srv-1", refetch),
    );
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.deleteDomain("gone.example.com");
    });
    expect(ok).toBe(true);
    expect(refetch).toHaveBeenCalled();
    expect(toast.error).not.toHaveBeenCalled();
  });

  it("still reports any other refusal", async () => {
    deleteMutation.mockRejectedValue(codedGraphQLError("FORBIDDEN"));
    const { result } = renderHook(() =>
      useCustomDomainMutations("srv-1", refetch),
    );
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.deleteDomain("kept.example.com");
    });
    expect(ok).toBe(false);
    expect(toast.error).toHaveBeenCalled();
  });
});
