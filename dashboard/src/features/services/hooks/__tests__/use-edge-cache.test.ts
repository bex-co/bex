import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { CombinedGraphQLErrors } from "@apollo/client/errors";

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

import { useEdgeCache } from "@/features/services/hooks/use-edge-cache";

beforeEach(() => {
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useEdgeCache", () => {
  it("sends setCacheProfile with the chosen profile and reports success", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockReturnValue([mutate]);
    const { result } = renderHook(() => useEdgeCache());

    await act(async () => {
      expect(
        await result.current.setCacheProfile("srv-web", "origin-controlled"),
      ).toBe(true);
    });

    expect(mutate).toHaveBeenCalledWith({
      variables: { id: "srv-web", profile: "origin-controlled" },
    });
    expect(toastSuccess).toHaveBeenCalledWith("Edge caching updated.");
  });

  it("surfaces bex-api's free-plan refusal instead of a false success", async () => {
    mockUseMutation.mockReturnValue([
      vi.fn().mockRejectedValue(
        new CombinedGraphQLErrors({
          errors: [
            {
              message:
                "edge caching is not available for free web services; choose a paid plan first",
            },
          ],
        } as never),
      ),
    ]);
    const { result } = renderHook(() => useEdgeCache());

    await act(async () => {
      expect(
        await result.current.setCacheProfile("srv-web", "origin-controlled"),
      ).toBe(false);
    });

    expect(toastSuccess).not.toHaveBeenCalled();
    expect(toastError).toHaveBeenCalledWith(
      expect.stringContaining("not available for free web services"),
    );
  });

  it("purges and notes the cache restarts with the service's instances", async () => {
    const mutate = vi.fn().mockResolvedValue({});
    mockUseMutation.mockReturnValue([mutate]);
    const { result } = renderHook(() => useEdgeCache());

    await act(async () => {
      expect(await result.current.purgeCache("srv-web")).toBe(true);
    });

    expect(mutate).toHaveBeenCalledWith({ variables: { id: "srv-web" } });
    expect(toastSuccess).toHaveBeenCalledWith(
      "Cache purged. It restarts with your service's instances.",
    );
  });
});
