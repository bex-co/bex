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

// w4/618: the toasts name domains as saved, and both halves of an auto-pair.
describe("useCustomDomainMutations toasts name the saved domains", () => {
  it("names an added domain as saved, not as typed", async () => {
    deleteMutation.mockResolvedValue({
      data: { addCustomDomain: { name: "qa-example.com" } },
    });
    refetch.mockResolvedValue([{ name: "qa-example.com" }]);
    const { result } = renderHook(() =>
      useCustomDomainMutations("srv-1", refetch),
    );
    await act(async () => {
      await result.current.addDomain("Qa-Example.COM.");
    });
    expect(toast.success).toHaveBeenCalledWith(
      "Added qa-example.com",
      expect.anything(),
    );
  });

  it("names the auto-paired www sibling an apex add created", async () => {
    deleteMutation.mockResolvedValue({
      data: { addCustomDomain: { name: "example.com" } },
    });
    refetch.mockResolvedValue([
      { name: "example.com" },
      { name: "www.example.com", redirectForName: "example.com" },
    ]);
    const { result } = renderHook(() =>
      useCustomDomainMutations("srv-1", refetch),
    );
    await act(async () => {
      await result.current.addDomain("example.com");
    });
    expect(toast.success).toHaveBeenCalledWith(
      "Added example.com and www.example.com",
      expect.anything(),
    );
  });

  it("names the sibling a delete removed with it, and only then", async () => {
    deleteMutation.mockResolvedValue({});
    refetch.mockResolvedValue([]);
    const { result } = renderHook(() =>
      useCustomDomainMutations("srv-1", refetch),
    );
    await act(async () => {
      await result.current.deleteDomain("example.com", "www.example.com");
    });
    expect(toast.success).toHaveBeenLastCalledWith(
      "Removed example.com and www.example.com",
    );

    refetch.mockResolvedValue([{ name: "example.com" }]);
    await act(async () => {
      await result.current.deleteDomain("www.example.com", "example.com");
    });
    expect(toast.success).toHaveBeenLastCalledWith("Removed www.example.com");
  });
});
