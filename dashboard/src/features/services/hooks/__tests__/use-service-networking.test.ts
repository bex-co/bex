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

import { uncodedGraphQLError } from "@/test/mocks/apollo";
import { useServiceNetworking } from "@/features/services/hooks/use-service-networking";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useServiceNetworking", () => {
  it("submits complete description-preserving entries", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockReturnValue([mutate, { loading: false }]);
    const { result } = renderHook(() => useServiceNetworking());
    const entries = [{ cidrBlock: "203.0.113.0/24", description: "office" }];

    await act(async () => {
      expect(await result.current.saveAllowList("srv-web", entries)).toBe(true);
    });
    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "srv-web", entries },
    });
    expect(toastSuccess).toHaveBeenCalledWith("IP allowlist updated");
  });

  it("reports mutation failures without false success", async () => {
    mockUseMutation.mockReturnValue([
      vi
        .fn()
        .mockRejectedValue(uncodedGraphQLError("bad request: invalid CIDR")),
      { loading: false },
    ]);
    const { result } = renderHook(() => useServiceNetworking());
    await act(async () => {
      expect(await result.current.saveAllowList("srv-web", [])).toBe(false);
    });
    expect(toastError).toHaveBeenCalledWith("Invalid CIDR");
    expect(toastSuccess).not.toHaveBeenCalled();
  });

  it("shows reason-less copy for a transport failure (w5/166)", async () => {
    mockUseMutation.mockReturnValue([
      vi.fn().mockRejectedValue(new TypeError("Failed to fetch")),
      { loading: false },
    ]);
    const { result } = renderHook(() => useServiceNetworking());
    await act(async () => {
      expect(await result.current.saveAllowList("srv-web", [])).toBe(false);
    });
    expect(toastError).toHaveBeenCalledWith(
      "Failed to update IP allowlist. Please try again.",
    );
  });
});
