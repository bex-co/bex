import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

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
    success: (...args: unknown[]) => toastSuccess(...args),
    error: (...args: unknown[]) => toastError(...args),
  },
}));

import { useDeployHook } from "@/features/services/hooks/use-deploy-hook";

beforeEach(() => {
  mockUseQuery.mockReset();
  mockUseMutation.mockReset();
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("useDeployHook", () => {
  it("reads the service's stable hook URL", async () => {
    mockUseQuery.mockReturnValue({
      data: { deployHook: { url: "https://api.bex.co/v1/deploy-hooks/old" } },
      loading: false,
      error: undefined,
    });
    mockUseMutation.mockReturnValue([vi.fn()]);

    const { result } = renderHook(() => useDeployHook("web"));
    await waitFor(() =>
      expect(result.current.url).toBe("https://api.bex.co/v1/deploy-hooks/old"),
    );
    expect(mockUseQuery).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ variables: { serviceId: "web" } }),
    );
  });

  it("replaces the invalidated URL immediately after regeneration", async () => {
    mockUseQuery.mockReturnValue({
      data: { deployHook: { url: "https://api.bex.co/v1/deploy-hooks/old" } },
      loading: false,
      error: undefined,
    });
    const mutate = vi.fn().mockResolvedValue({
      data: {
        regenerateDeployHook: {
          url: "https://api.bex.co/v1/deploy-hooks/new",
        },
      },
    });
    mockUseMutation.mockReturnValue([mutate]);

    const { result } = renderHook(() => useDeployHook("web"));
    await waitFor(() => expect(result.current.url).toContain("/old"));
    let ok = false;
    await act(async () => {
      ok = await result.current.regenerate();
    });

    expect(ok).toBe(true);
    expect(mutate).toHaveBeenCalledWith({ variables: { serviceId: "web" } });
    expect(result.current.url).toBe("https://api.bex.co/v1/deploy-hooks/new");
    expect(toastSuccess).toHaveBeenCalledWith(
      "Deploy Hook regenerated. The old URL no longer works.",
    );
  });

  it("keeps the current URL and reports a failed regeneration", async () => {
    mockUseQuery.mockReturnValue({
      data: { deployHook: { url: "https://api.bex.co/v1/deploy-hooks/old" } },
      loading: false,
      error: undefined,
    });
    mockUseMutation.mockReturnValue([
      vi.fn().mockRejectedValue(new Error("forbidden")),
    ]);

    const { result } = renderHook(() => useDeployHook("web"));
    await waitFor(() => expect(result.current.url).toContain("/old"));
    let ok = true;
    await act(async () => {
      ok = await result.current.regenerate();
    });

    expect(ok).toBe(false);
    expect(result.current.url).toContain("/old");
    expect(toastError).toHaveBeenCalledWith(
      "Couldn't regenerate the Deploy Hook. Please try again.",
    );
  });
});
