import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const queryState: {
  data: unknown;
  loading: boolean;
  options: Record<string, unknown> | null;
} = { data: undefined, loading: false, options: null };

vi.mock("@apollo/client/react", () => ({
  useQuery: (_document: unknown, options: Record<string, unknown>) => {
    queryState.options = options;
    return { data: queryState.data, loading: queryState.loading };
  },
}));

let debouncedRootDir = "";
vi.mock("@/common/hooks/use-debounce", () => ({
  useDebounce: () => debouncedRootDir,
}));

let currentWorkspaceId: string | null = "tea-1";
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId }),
}));

import { useRepoRuntimeDetection } from "@/features/services/hooks/use-repo-runtime-detection";

describe("useRepoRuntimeDetection", () => {
  beforeEach(() => {
    queryState.data = undefined;
    queryState.loading = false;
    queryState.options = null;
    debouncedRootDir = "";
    currentWorkspaceId = "tea-1";
  });

  it("queries the selected workspace and returns a validated runtime", () => {
    debouncedRootDir = "services/api";
    queryState.data = {
      repoRuntimeDetection: {
        runtime: "go",
        matchedManifest: "go.mod",
      },
    };
    const { result } = renderHook(() =>
      useRepoRuntimeDetection({
        repo: "https://github.com/acme/mono",
        branch: "main",
        rootDir: "services/api",
      }),
    );

    expect(queryState.options).toMatchObject({
      variables: {
        repo: "https://github.com/acme/mono",
        branch: "main",
        rootDir: "services/api",
        ownerId: "tea-1",
      },
      skip: false,
      fetchPolicy: "network-only",
      errorPolicy: "all",
    });
    expect(result.current).toBe("go");
  });

  it("withholds stale, loading, unknown, and invalid results", () => {
    queryState.data = {
      repoRuntimeDetection: {
        runtime: "made-up-runtime",
        matchedManifest: "mystery.lock",
      },
    };
    queryState.loading = true;
    debouncedRootDir = "services/old";
    const { result, rerender } = renderHook(
      ({ rootDir }) =>
        useRepoRuntimeDetection({
          repo: "https://github.com/acme/mono",
          branch: "main",
          rootDir,
        }),
      { initialProps: { rootDir: "services/new" } },
    );

    expect(result.current).toBeNull();

    debouncedRootDir = "services/new";
    queryState.loading = false;
    rerender({ rootDir: "services/new" });
    expect(result.current).toBeNull();

    queryState.data = undefined;
    rerender({ rootDir: "services/new" });
    expect(result.current).toBeNull();
  });

  it("skips until repo, branch, and workspace are available", () => {
    currentWorkspaceId = null;
    const { result } = renderHook(() =>
      useRepoRuntimeDetection({ repo: null, branch: "", rootDir: "" }),
    );
    expect(queryState.options).toMatchObject({ skip: true });
    expect(result.current).toBeNull();
  });
});
