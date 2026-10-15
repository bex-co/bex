import { act, renderHook, waitFor } from "@testing-library/react";
import { useLayoutEffect } from "react";
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

  it("follows the read's URL without lagging it, and keeps it through a failed refresh", () => {
    const OLD = "https://api.bex.co/v1/deploy-hooks/old";
    const NEW = "https://api.bex.co/v1/deploy-hooks/new";
    mockUseMutation.mockReturnValue([vi.fn()]);
    let read: { data?: unknown; error?: Error } = {
      data: { deployHook: { url: OLD } },
    };
    mockUseQuery.mockImplementation(() => ({ loading: false, ...read }));
    const committed: (string | null)[] = [];
    const { result, rerender } = renderHook(() => {
      const value = useDeployHook("web");
      useLayoutEffect(() => {
        committed.push(value.url);
      });
      return value;
    });
    expect(result.current.url).toBe(OLD);

    read = { data: undefined, error: new Error("ERR_CONNECTION_CLOSED") };
    rerender();
    expect(result.current.url).toBe(OLD);

    read = { data: { deployHook: { url: NEW } } };
    committed.length = 0;
    rerender();
    expect(result.current.url).toBe(NEW);
    // The rotated-away URL is never on screen once the read has the new one.
    expect(committed).toEqual([NEW]);
  });

  // w1/130: the URL is a credential for one service. A section that moves to
  // another service in place never shows the previous service's URL, not even
  // while the new read is loading or after it fails.
  it("never shows one service's URL under another", () => {
    const WEB = "https://api.bex.co/v1/deploy-hooks/web";
    const API = "https://api.bex.co/v1/deploy-hooks/api";
    mockUseMutation.mockReturnValue([vi.fn()]);
    const reads: Record<string, { data?: unknown; error?: Error }> = {
      web: { data: { deployHook: { url: WEB } } },
      api: {},
    };
    mockUseQuery.mockImplementation(
      (_doc: unknown, opts: { variables: { serviceId: string } }) => ({
        loading: false,
        ...reads[opts.variables.serviceId],
      }),
    );
    const committed: (string | null)[] = [];
    const { result, rerender } = renderHook(
      ({ serviceId }) => {
        const value = useDeployHook(serviceId);
        useLayoutEffect(() => {
          committed.push(value.url);
        });
        return value;
      },
      { initialProps: { serviceId: "web" } },
    );
    expect(result.current.url).toBe(WEB);

    committed.length = 0;
    rerender({ serviceId: "api" });
    reads.api = { error: new Error("ERR_CONNECTION_CLOSED") };
    rerender({ serviceId: "api" });
    expect(committed).toEqual([null, null]);

    reads.api = { data: { deployHook: { url: API } } };
    rerender({ serviceId: "api" });
    expect(result.current.url).toBe(API);
    expect(committed).not.toContain(WEB);
  });

  it("drops a rotation that completes after the section moved on", async () => {
    const WEB = "https://api.bex.co/v1/deploy-hooks/web";
    let finish: (value: unknown) => void = () => undefined;
    mockUseMutation.mockReturnValue([
      () => new Promise((resolve) => (finish = resolve)),
    ]);
    mockUseQuery.mockImplementation(
      (_doc: unknown, opts: { variables: { serviceId: string } }) => ({
        loading: false,
        data:
          opts.variables.serviceId === "web"
            ? { deployHook: { url: WEB } }
            : undefined,
      }),
    );
    const { result, rerender } = renderHook(
      ({ serviceId }) => useDeployHook(serviceId),
      { initialProps: { serviceId: "web" } },
    );
    let rotated: Promise<boolean> = Promise.resolve(false);
    act(() => {
      rotated = result.current.regenerate();
    });
    rerender({ serviceId: "api" });
    await act(async () => {
      finish({
        data: {
          regenerateDeployHook: {
            url: "https://api.bex.co/v1/deploy-hooks/web-rotated",
          },
        },
      });
      await rotated;
    });
    expect(result.current.url).toBeNull();
  });
});
