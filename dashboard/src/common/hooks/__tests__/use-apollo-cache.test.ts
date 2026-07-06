import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { useApolloCacheClear } from "../use-apollo-cache";

// Mock @apollo/client/react
const mockEvict = vi.fn();
const mockApolloClient = {
  cache: {
    evict: mockEvict,
  },
};

vi.mock("@apollo/client/react", () => ({
  useApolloClient: () => mockApolloClient,
}));

describe("useApolloCacheClear", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("should return a function", () => {
    const { result } = renderHook(() => useApolloCacheClear());

    expect(typeof result.current).toBe("function");
  });

  it("should call cache.evict with ROOT_QUERY when clearCache is called", () => {
    const { result } = renderHook(() => useApolloCacheClear());

    result.current();

    expect(mockEvict).toHaveBeenCalledTimes(1);
    expect(mockEvict).toHaveBeenCalledWith({ id: "ROOT_QUERY" });
  });

  it("should return the same function reference on re-render (memoization)", () => {
    const { result, rerender } = renderHook(() => useApolloCacheClear());

    const firstRenderFunction = result.current;

    rerender();

    const secondRenderFunction = result.current;

    expect(firstRenderFunction).toBe(secondRenderFunction);
  });

  it("should allow multiple calls to clearCache", () => {
    const { result } = renderHook(() => useApolloCacheClear());

    result.current();
    result.current();
    result.current();

    expect(mockEvict).toHaveBeenCalledTimes(3);
  });

  it("should not call evict until the returned function is invoked", () => {
    renderHook(() => useApolloCacheClear());

    expect(mockEvict).not.toHaveBeenCalled();
  });
});
