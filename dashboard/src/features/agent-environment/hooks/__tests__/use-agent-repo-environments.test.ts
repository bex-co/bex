import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AgentRepoEnvironmentsDocument } from "@/graphql/definitions";
import { useAgentRepoEnvironments } from "@/features/agent-environment/hooks/use-agent-repo-environments";
import { BUILD_POLL_INTERVAL_MS } from "@/features/agent-environment/lib/repo-environment-builds";

const mutate = vi.fn();
const startPolling = vi.fn();
const stopPolling = vi.fn();
const latest = vi.hoisted(() => ({ status: null as string | null }));

function latestBuild(status: string) {
  return {
    __typename: "AgentRepoEnvironmentBuild",
    id: "arb-1",
    ownerId: "tea-1",
    repo: "acme/api",
    version: "v1",
    trigger: "environment_saved",
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

vi.mock("@apollo/client/react", () => ({
  useQuery: () => ({
    data: {
      agentRepoEnvironments: {
        __typename: "AgentRepoEnvironmentList",
        ownerId: "tea-1",
        environments: [
          {
            __typename: "AgentRepoEnvironment",
            repo: "acme/api",
            version: "v1",
            setup: [
              { __typename: "AgentRepoStep", name: "deps", run: "npm ci" },
            ],
            checks: [
              { __typename: "AgentRepoStep", name: "test", run: "npm test" },
            ],
            latestBuild: latest.status ? latestBuild(latest.status) : null,
          },
          {
            __typename: "AgentRepoEnvironment",
            repo: "acme/web",
            version: "v0",
            setup: [],
            checks: [
              { __typename: "AgentRepoStep", name: "test", run: "npm test" },
            ],
            latestBuild: null,
          },
        ],
      },
    },
    loading: false,
    error: undefined,
    refetch: vi.fn(),
    startPolling,
    stopPolling,
  }),
  useMutation: () => [mutate, { loading: false }],
}));

describe("useAgentRepoEnvironments", () => {
  beforeEach(() => {
    mutate.mockReset();
    mutate.mockResolvedValue({ data: {} });
    startPolling.mockReset();
    stopPolling.mockReset();
    latest.status = null;
  });

  // The list feeds the editor, whose save goes back as mutation input: a
  // __typename riding along would be refused as an unknown input field.
  it("reads environments as plain steps, with their version and newest build", () => {
    latest.status = "success";
    const { result } = renderHook(() => useAgentRepoEnvironments("tea-1"));
    expect(result.current.environments).toStrictEqual([
      {
        repo: "acme/api",
        version: "v1",
        setup: [{ name: "deps", run: "npm ci" }],
        checks: [{ name: "test", run: "npm test" }],
        latestBuild: latestBuild("success"),
      },
      {
        repo: "acme/web",
        version: "v0",
        setup: [],
        checks: [{ name: "test", run: "npm test" }],
        latestBuild: null,
      },
    ]);
  });

  it("sends the whole environment and refreshes the list", async () => {
    const { result } = renderHook(() => useAgentRepoEnvironments("tea-1"));
    const environment = {
      setup: [{ name: "deps", run: "npm ci" }],
      checks: [],
    };
    await act(() => result.current.set("acme/api", environment));
    expect(mutate).toHaveBeenLastCalledWith({
      variables: { ownerId: "tea-1", repo: "acme/api", ...environment },
      refetchQueries: [AgentRepoEnvironmentsDocument],
      awaitRefetchQueries: true,
    });
    await act(() => result.current.remove("acme/api"));
    expect(mutate).toHaveBeenLastCalledWith({
      variables: { ownerId: "tea-1", repo: "acme/api" },
      refetchQueries: [AgentRepoEnvironmentsDocument],
      awaitRefetchQueries: true,
    });
  });

  // ADR111: a queued or building build re-reads the list every 10 s so the
  // row follows it; once none is active no timer runs.
  it("polls only while a repository's newest build is queued or building", () => {
    const { rerender } = renderHook(() => useAgentRepoEnvironments("tea-1"));
    expect(startPolling).not.toHaveBeenCalled();

    latest.status = "queued";
    rerender();
    expect(startPolling).toHaveBeenCalledTimes(1);
    expect(startPolling).toHaveBeenLastCalledWith(BUILD_POLL_INTERVAL_MS);
    expect(BUILD_POLL_INTERVAL_MS).toBe(10_000);

    // Queued → building keeps the one timer.
    latest.status = "building";
    rerender();
    expect(startPolling).toHaveBeenCalledTimes(1);
    expect(stopPolling).not.toHaveBeenCalled();

    latest.status = "failed";
    rerender();
    expect(stopPolling).toHaveBeenCalledTimes(1);
    expect(startPolling).toHaveBeenCalledTimes(1);
  });

  it("stops polling when the card goes away mid-build", () => {
    latest.status = "building";
    const { unmount } = renderHook(() => useAgentRepoEnvironments("tea-1"));
    expect(startPolling).toHaveBeenCalledTimes(1);
    unmount();
    expect(stopPolling).toHaveBeenCalledTimes(1);
  });
});
