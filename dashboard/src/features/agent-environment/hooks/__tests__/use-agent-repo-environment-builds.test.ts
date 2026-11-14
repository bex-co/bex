import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  AgentRepoEnvironmentBuildsDocument,
  AgentRepoEnvironmentsDocument,
} from "@/graphql/definitions";
import {
  useAgentRepoEnvironmentBuild,
  useAgentRepoEnvironmentBuilds,
} from "@/features/agent-environment/hooks/use-agent-repo-environment-builds";
import { BUILD_POLL_INTERVAL_MS } from "@/features/agent-environment/lib/repo-environment-builds";

const mutate = vi.fn();
const startPolling = vi.fn();
const stopPolling = vi.fn();
const refetch = vi.fn();
const state = vi.hoisted(() => ({
  statuses: [] as string[],
  loading: false,
  lastOptions: null as null | {
    variables: Record<string, unknown>;
    skip?: boolean;
  },
}));

function build(id: string, status: string) {
  return {
    __typename: "AgentRepoEnvironmentBuild",
    id,
    ownerId: "tea-1",
    repo: "acme/api",
    version: "v1",
    trigger: "manual",
    status,
    failureCode: null,
    failureReason: null,
    headSha: null,
    sizeBytes: null,
    imageAvailable: false,
    createdAt: "2026-11-14T09:00:00Z",
    startedAt: null,
    finishedAt: null,
  };
}

vi.mock("@apollo/client/react", async () => {
  const defs = await import("@/graphql/definitions");
  return {
    useQuery: (
      document: unknown,
      options: { variables: Record<string, unknown>; skip?: boolean },
    ) => {
      state.lastOptions = options;
      const base = {
        loading: state.loading,
        error: undefined,
        refetch,
        startPolling,
        stopPolling,
      };
      if (state.loading) return { ...base, data: undefined };
      if (document === defs.AgentRepoEnvironmentBuildsDocument) {
        return {
          ...base,
          data: {
            agentRepoEnvironmentBuilds: {
              __typename: "AgentRepoEnvironmentBuildList",
              ownerId: "tea-1",
              repo: "acme/api",
              builds: state.statuses.map((status, i) =>
                build(`arb-${i + 1}`, status),
              ),
            },
          },
        };
      }
      if (options.skip) return { ...base, data: undefined };
      return {
        ...base,
        data: {
          agentRepoEnvironmentBuild: {
            ...build(String(options.variables.id), state.statuses[0] ?? ""),
            steps: [],
          },
        },
      };
    },
    useMutation: () => [mutate, { loading: false }],
  };
});

beforeEach(() => {
  mutate.mockReset();
  startPolling.mockReset();
  stopPolling.mockReset();
  refetch.mockReset();
  state.statuses = [];
  state.loading = false;
  state.lastOptions = null;
});

describe("useAgentRepoEnvironmentBuilds", () => {
  it("reads the repository's builds, newest first", () => {
    state.statuses = ["failed", "success"];
    const { result } = renderHook(() =>
      useAgentRepoEnvironmentBuilds("tea-1", "acme/api"),
    );
    expect(state.lastOptions?.variables).toEqual({
      ownerId: "tea-1",
      repo: "acme/api",
    });
    expect(result.current.builds.map((b) => [b.id, b.status])).toEqual([
      ["arb-1", "failed"],
      ["arb-2", "success"],
    ]);
    expect(result.current.loading).toBe(false);
  });

  it("is loading only until the first list arrives", () => {
    state.loading = true;
    const { result } = renderHook(() =>
      useAgentRepoEnvironmentBuilds("tea-1", "acme/api"),
    );
    expect(result.current.loading).toBe(true);
    expect(result.current.builds).toEqual([]);
  });

  it("polls only while one of the builds is queued or building", () => {
    state.statuses = ["success"];
    const { rerender, unmount } = renderHook(() =>
      useAgentRepoEnvironmentBuilds("tea-1", "acme/api"),
    );
    expect(startPolling).not.toHaveBeenCalled();
    state.statuses = ["queued", "building", "success"];
    rerender();
    expect(startPolling).toHaveBeenCalledWith(BUILD_POLL_INTERVAL_MS);
    state.statuses = ["cancelled", "success", "success"];
    rerender();
    expect(stopPolling).toHaveBeenCalledTimes(1);
    unmount();
    expect(stopPolling).toHaveBeenCalledTimes(1);
  });

  // Build now refreshes the row's newest build and this history.
  it("queues a build and returns it", async () => {
    mutate.mockResolvedValue({
      data: { buildAgentRepoEnvironment: build("arb-9", "queued") },
    });
    const { result } = renderHook(() =>
      useAgentRepoEnvironmentBuilds("tea-1", "acme/api"),
    );
    let queued: unknown;
    await act(async () => {
      queued = await result.current.buildNow();
    });
    expect(mutate).toHaveBeenCalledWith({
      variables: { ownerId: "tea-1", repo: "acme/api" },
      refetchQueries: [
        AgentRepoEnvironmentsDocument,
        AgentRepoEnvironmentBuildsDocument,
      ],
      awaitRefetchQueries: true,
    });
    expect(queued).toEqual(build("arb-9", "queued"));
  });

  it("passes a refusal through to the caller", async () => {
    const refusal = new Error("rate limited");
    mutate.mockRejectedValue(refusal);
    const { result } = renderHook(() =>
      useAgentRepoEnvironmentBuilds("tea-1", "acme/api"),
    );
    await expect(result.current.buildNow()).rejects.toBe(refusal);
  });
});

describe("useAgentRepoEnvironmentBuild", () => {
  it("reads nothing until a build is selected", () => {
    const { result } = renderHook(() =>
      useAgentRepoEnvironmentBuild("tea-1", null),
    );
    expect(state.lastOptions?.skip).toBe(true);
    expect(result.current.build).toBeNull();
    expect(result.current.loading).toBe(false);
    expect(startPolling).not.toHaveBeenCalled();
  });

  it("reads one build with its log", () => {
    state.statuses = ["success"];
    const { result } = renderHook(() =>
      useAgentRepoEnvironmentBuild("tea-1", "arb-1"),
    );
    expect(state.lastOptions).toMatchObject({
      variables: { ownerId: "tea-1", id: "arb-1" },
      skip: false,
    });
    expect(result.current.build?.id).toBe("arb-1");
    expect(result.current.build?.steps).toEqual([]);
    expect(startPolling).not.toHaveBeenCalled();
  });

  // Another query can settle the build's status in the shared cache first,
  // so the log is read once more after the poll stops.
  it("polls a running build, then reads its final log once", () => {
    state.statuses = ["building"];
    const { rerender, unmount } = renderHook(() =>
      useAgentRepoEnvironmentBuild("tea-1", "arb-1"),
    );
    expect(startPolling).toHaveBeenCalledWith(BUILD_POLL_INTERVAL_MS);
    expect(refetch).not.toHaveBeenCalled();
    state.statuses = ["success"];
    rerender();
    expect(stopPolling).toHaveBeenCalledTimes(1);
    expect(refetch).toHaveBeenCalledTimes(1);
    rerender();
    expect(refetch).toHaveBeenCalledTimes(1);
    unmount();
    expect(startPolling).toHaveBeenCalledTimes(1);
  });
});
