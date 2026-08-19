import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useAgentSessionMutations } from "@/features/agent-sessions/hooks/use-agent-session-mutations";

const mutate = vi.fn();
const evict = vi.fn();
const gc = vi.fn();
const writeQuery = vi.fn();

vi.mock("@apollo/client/react", () => ({
  useApolloClient: () => ({ cache: { evict, gc }, writeQuery }),
  useMutation: () => [mutate],
}));

describe("useAgentSessionMutations list cache invalidation", () => {
  beforeEach(() => {
    mutate.mockReset();
    evict.mockReset();
    gc.mockReset();
    writeQuery.mockReset();
  });

  it("writes a lifecycle result into detail cache and expires every list", async () => {
    const session = {
      __typename: "AgentSession",
      id: "ags-1",
      ownerId: "tea-1",
      repo: "acme/widgets",
      branch: "bex-agent/fix",
      sandboxId: null,
      sshAddress: null,
      phase: "completed",
      status: "done",
      headSha: null,
      prUrl: null,
      prNumber: null,
      turns: 1,
      deliveryMode: null,
      failureReason: null,
      createdAt: "2026-08-18T00:00:00Z",
      updatedAt: "2026-08-18T01:00:00Z",
      canceledAt: null,
      pinned: false,
      snapshotBytes: 0,
      hibernatedAt: null,
      retainUntil: null,
      archivedAt: "2026-08-18T01:00:00Z",
      agentConfig: {
        __typename: "AgentSessionConfig",
        agent: "codex",
        model: null,
        modelEndpoint: null,
        task: "fix it",
        template: null,
      },
    };
    mutate.mockResolvedValue({ data: { archiveAgentSession: session } });
    const { result } = renderHook(() => useAgentSessionMutations());

    await act(() => result.current.archive("ags-1"));

    expect(writeQuery).toHaveBeenCalledWith(
      expect.objectContaining({
        variables: { id: "ags-1" },
        data: { agentSession: session },
      }),
    );
    expect(evict).toHaveBeenCalledWith({
      id: "ROOT_QUERY",
      fieldName: "agentSessions",
    });
  });

  it("evicts every agentSessions query variant after a successful delete", async () => {
    mutate.mockResolvedValue({ data: { deleteAgentSession: true } });
    const { result } = renderHook(() => useAgentSessionMutations());

    await act(() => result.current.deleteSession("ags-1"));

    expect(evict).toHaveBeenCalledWith({
      id: "ROOT_QUERY",
      fieldName: "agentSessions",
    });
    expect(evict).toHaveBeenCalledWith({
      id: "ROOT_QUERY",
      fieldName: "agentSession",
      args: { id: "ags-1" },
    });
    expect(gc).toHaveBeenCalledTimes(1);
  });

  it("keeps list data intact when a delete fails", async () => {
    mutate.mockRejectedValue(new Error("offline"));
    const { result } = renderHook(() => useAgentSessionMutations());

    await expect(result.current.deleteSession("ags-1")).rejects.toThrow(
      "offline",
    );
    expect(evict).not.toHaveBeenCalled();
  });
});
