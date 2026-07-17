import { describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import { useLatestDeploy } from "../use-latest-deploy";

const useQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => useQuery(...args),
}));

describe("useLatestDeploy", () => {
  it("requests one newest deploy and maps only header facts", () => {
    useQuery.mockReturnValue({
      data: {
        deploys: [{ id: "dep-new", status: "build_failed" }],
      },
      loading: false,
      error: undefined,
    });

    const { result } = renderHook(() => useLatestDeploy("web"));

    expect(useQuery).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({ variables: { serviceId: "web", limit: 1 } }),
    );
    expect(result.current.deploy).toEqual({
      id: "dep-new",
      status: "build_failed",
    });
  });

  it("returns null for an empty history", () => {
    useQuery.mockReturnValue({ data: { deploys: [] }, loading: false });

    const { result } = renderHook(() => useLatestDeploy("web"));

    expect(result.current.deploy).toBeNull();
  });
});
