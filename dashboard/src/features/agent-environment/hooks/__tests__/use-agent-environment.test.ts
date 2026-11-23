import { act, renderHook } from "@testing-library/react";
import { InMemoryCache } from "@apollo/client";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  AgentNetworkTier,
  AgentSessionCapabilitiesDocument,
} from "@/graphql/definitions";
import { useAgentEnvironment } from "@/features/agent-environment/hooks/use-agent-environment";

const mutate = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: () => ({
    data: undefined,
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
  useMutation: () => [mutate, { loading: false }],
}));

/**
 * Runs the last save's cache update over the composer's capabilities read,
 * as its cache-first readers leave it; true while it stays cached.
 */
function capabilitiesKeptAfterSave(): boolean {
  const cache = new InMemoryCache();
  const read = {
    query: AgentSessionCapabilitiesDocument,
    variables: { ownerId: "tea-1" },
  };
  cache.writeQuery({
    ...read,
    data: {
      agentSessionCapabilities: {
        __typename: "AgentSessionCapabilities",
        enabled: true,
        sshEnabled: false,
        defaultTemplate: "agent",
        agents: [],
        templates: [],
        github: { __typename: "AgentSessionGitHubReadiness", connected: true },
        attachments: {
          __typename: "AgentAttachmentLimits",
          enabled: false,
          maxBytes: 0,
          maxPerTurn: 0,
        },
      },
    },
  });
  const { update } = mutate.mock.lastCall![0] as {
    update?: (cache: InMemoryCache) => void;
  };
  update?.(cache);
  return cache.readQuery(read) != null;
}

describe("useAgentEnvironment update", () => {
  beforeEach(() => {
    mutate.mockReset();
    mutate.mockResolvedValue({
      data: { updateAgentEnvironment: { ownerId: "tea-1" } },
    });
  });

  // The composer's per-agent model-key readiness, default model, the models
  // members may pick (w10/m527) and the default sandbox come from the
  // capabilities read, which nothing on Settings watches; a save of any of
  // them evicts it so the next composer or automation form reads it afresh,
  // and other saves leave it cached.
  it("forgets the composer capabilities only when a save changes what they carry", async () => {
    const { result } = renderHook(() => useAgentEnvironment("tea-1"));
    for (const input of [
      { modelApiKey: "sk-ant-api03-x" },
      { clearModelApiKey: true },
      { defaultModel: "anthropic/claude-sonnet-5.5" },
      { defaultTemplate: "agent-8c-16g" },
      { defaultTemplate: "" },
      {
        enabledModels: [
          "deepseek/deepseek-v4.1-flash-1017",
          "anthropic/claude-haiku-5.5",
        ],
      },
    ]) {
      await act(() => result.current.update(input));
      expect(mutate).toHaveBeenLastCalledWith({
        variables: { ownerId: "tea-1", input },
        update: expect.any(Function),
      });
      expect(capabilitiesKeptAfterSave()).toBe(false);
    }
    // The capabilities carry no network tier and no gh switch (w10/m524).
    for (const input of [
      { networkTier: AgentNetworkTier.Full },
      { githubCliEnabled: false },
      { githubCliEnabled: true },
    ]) {
      await act(() => result.current.update(input));
      expect(mutate).toHaveBeenLastCalledWith({
        variables: { ownerId: "tea-1", input },
        update: undefined,
      });
      expect(capabilitiesKeptAfterSave()).toBe(true);
    }
  });
});
