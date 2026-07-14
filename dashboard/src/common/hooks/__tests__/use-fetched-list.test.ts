import { describe, it, expect, vi, afterEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { useFetchedList } from "@/common/hooks/use-fetched-list";

function stubFetch(impl: () => Response) {
  vi.stubGlobal("fetch", vi.fn(async () => impl()));
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("useFetchedList", () => {
  it("loads the list on mount", async () => {
    stubFetch(() => Response.json([{ id: "a" }]));
    const { result } = renderHook(() => useFetchedList<{ id: string }>("/x"));

    expect(result.current.loading).toBe(true);
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.data).toEqual([{ id: "a" }]);
    expect(result.current.error).toBe(false);
  });

  it("sets error on a non-ok response", async () => {
    stubFetch(() => new Response("boom", { status: 500 }));
    const { result } = renderHook(() => useFetchedList<{ id: string }>("/x"));

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBe(true);
    expect(result.current.data).toEqual([]);
  });

  it("setData lets a caller apply an optimistic update", async () => {
    stubFetch(() => Response.json([{ id: "a" }, { id: "b" }]));
    const { result } = renderHook(() => useFetchedList<{ id: string }>("/x"));
    await waitFor(() => expect(result.current.loading).toBe(false));

    act(() => {
      result.current.setData((prev) => prev.filter((x) => x.id !== "a"));
    });

    expect(result.current.data).toEqual([{ id: "b" }]);
  });

  it("refetch re-runs the fetch", async () => {
    let calls = 0;
    stubFetch(() => {
      calls += 1;
      return Response.json([{ id: `call-${calls}` }]);
    });
    const { result } = renderHook(() => useFetchedList<{ id: string }>("/x"));
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.data).toEqual([{ id: "call-1" }]);

    act(() => {
      result.current.refetch();
    });
    await waitFor(() => expect(result.current.data).toEqual([{ id: "call-2" }]));
  });
});
