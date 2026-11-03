import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";

const mockUseQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-a" }),
}));

import { useBuildSources } from "@/features/services/hooks/use-build-sources";

const source = (i: number) => ({
  __typename: "BuildSource",
  id: `bds-${i}`,
  name: `src-${i}`,
  cursor: `bds-${i}`,
  git: {
    __typename: "BuildSourceGit",
    repoUrl: `https://github.com/a/r${i}`,
    runtime: "node",
  },
  image: null,
});

beforeEach(() => vi.clearAllMocks());

// w2/m262 t002: the picker reaches every build source, not only Render's
// first page of 100.
describe("useBuildSources paging", () => {
  it("fetches the next page after a full one, from its last cursor", async () => {
    const fetchMore = vi.fn(async () => ({}));
    const page = Array.from({ length: 100 }, (_, i) => source(i));
    mockUseQuery.mockReturnValue({
      data: { buildSources: page },
      loading: false,
      error: undefined,
      fetchMore,
    });

    const { result } = renderHook(() => useBuildSources(true));

    expect(result.current.sources).toHaveLength(100);
    expect(result.current.sources[0].nativeRuntime).toBe("node");
    await waitFor(() => expect(fetchMore).toHaveBeenCalledTimes(1));
    const call = fetchMore.mock.calls[0] as unknown as [
      {
        variables: { cursor: string };
        updateQuery: (
          prev: unknown,
          opts: unknown,
        ) => { buildSources: unknown[] };
      },
    ];
    expect(call[0].variables.cursor).toBe("bds-99");
    const merged = call[0].updateQuery(
      { buildSources: page },
      { fetchMoreResult: { buildSources: [source(99), source(100)] } },
    );
    expect(merged.buildSources).toHaveLength(101);
  });

  it("stops after a short page", () => {
    const fetchMore = vi.fn();
    mockUseQuery.mockReturnValue({
      data: { buildSources: [source(1), source(2)] },
      loading: false,
      error: undefined,
      fetchMore,
    });

    renderHook(() => useBuildSources(true));

    expect(fetchMore).not.toHaveBeenCalled();
  });

  it("fetches each cursor once, even when the effect runs again", async () => {
    const fetchMore = vi.fn(async () => ({}));
    const page = Array.from({ length: 100 }, (_, i) => source(i));
    mockUseQuery.mockReturnValue({
      data: { buildSources: page },
      loading: false,
      error: undefined,
      fetchMore,
    });

    const { rerender } = renderHook(() => useBuildSources(true));
    rerender();
    rerender();

    await waitFor(() => expect(fetchMore).toHaveBeenCalledTimes(1));
  });
});
