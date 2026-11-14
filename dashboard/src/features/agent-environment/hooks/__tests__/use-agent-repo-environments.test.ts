import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AgentRepoEnvironmentsDocument } from "@/graphql/definitions";
import { useAgentRepoEnvironments } from "@/features/agent-environment/hooks/use-agent-repo-environments";

const mutate = vi.fn();
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
            setup: [
              { __typename: "AgentRepoStep", name: "deps", run: "npm ci" },
            ],
            checks: [
              { __typename: "AgentRepoStep", name: "test", run: "npm test" },
            ],
          },
        ],
      },
    },
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
  useMutation: () => [mutate, { loading: false }],
}));

describe("useAgentRepoEnvironments", () => {
  beforeEach(() => {
    mutate.mockReset();
    mutate.mockResolvedValue({ data: {} });
  });

  // The list feeds the editor, whose save goes back as mutation input: a
  // __typename riding along would be refused as an unknown input field.
  it("reads environments as plain steps", () => {
    const { result } = renderHook(() => useAgentRepoEnvironments("tea-1"));
    expect(result.current.environments).toStrictEqual([
      {
        repo: "acme/api",
        setup: [{ name: "deps", run: "npm ci" }],
        checks: [{ name: "test", run: "npm test" }],
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
});
