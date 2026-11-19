import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AgentNetworkTier } from "@/graphql/definitions";
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

describe("useAgentEnvironment update", () => {
  beforeEach(() => {
    mutate.mockReset();
    mutate.mockResolvedValue({
      data: { updateAgentEnvironment: { ownerId: "tea-1" } },
    });
  });

  // The composer's per-agent model-key readiness, default model and default
  // sandbox come from the capabilities query, so a save of any of them
  // refetches it; other saves leave it alone.
  it("refetches the composer capabilities only when a save changes what they carry", async () => {
    const { result } = renderHook(() => useAgentEnvironment("tea-1"));
    for (const input of [
      { modelApiKey: "sk-ant-api03-x" },
      { clearModelApiKey: true },
      { defaultModel: "anthropic/claude-sonnet-5.5" },
      { defaultTemplate: "agent-8c-16g" },
      { defaultTemplate: "" },
    ]) {
      await act(() => result.current.update(input));
      expect(mutate).toHaveBeenLastCalledWith({
        variables: { ownerId: "tea-1", input },
        refetchQueries: ["AgentSessionCapabilities"],
      });
    }
    await act(() =>
      result.current.update({ networkTier: AgentNetworkTier.Full }),
    );
    expect(mutate).toHaveBeenLastCalledWith({
      variables: {
        ownerId: "tea-1",
        input: { networkTier: AgentNetworkTier.Full },
      },
      refetchQueries: undefined,
    });
  });
});
