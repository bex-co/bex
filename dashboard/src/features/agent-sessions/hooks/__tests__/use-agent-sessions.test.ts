import { useLayoutEffect } from "react";
import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  useAgentSessions,
  type UseAgentSessionsOptions,
} from "@/features/agent-sessions/hooks/use-agent-sessions";

const fetchMore = vi.fn();
let listed: ReturnType<typeof wireSession>[] = [];
// The variables the hook's latest render queried with.
let queriedVariables: Record<string, unknown> | undefined;

vi.mock("@apollo/client/react", () => ({
  useQuery: (
    _document: unknown,
    options: { variables: Record<string, unknown> },
  ) => {
    queriedVariables = options.variables;
    return {
      data: { agentSessions: listed },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
      startPolling: vi.fn(),
      fetchMore,
    };
  },
}));

vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-1" }),
}));

function wireSession(id: string) {
  return {
    __typename: "AgentSession" as const,
    id,
    ownerId: "tea-1",
    repo: "acme/widgets",
    branch: `bex-agent/${id}`,
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
    archivedAt: null,
    agentConfig: {
      __typename: "AgentSessionConfig" as const,
      agent: "codex",
      model: null,
      modelEndpoint: null,
      task: "fix it",
      template: null,
    },
  };
}

// Every committed `hasMore`, i.e. what a "Load more" button actually showed.
let committedHasMore: boolean[] = [];

function useCommittedAgentSessions(options: UseAgentSessionsOptions) {
  const value = useAgentSessions(options);
  useLayoutEffect(() => {
    committedHasMore.push(value.hasMore);
  });
  return value;
}

describe("useAgentSessions pagination", () => {
  beforeEach(() => {
    fetchMore.mockReset();
    listed = [wireSession("ags-1"), wireSession("ags-2")];
    committedHasMore = [];
  });

  it("pages from the last session of the newest rendered list", async () => {
    fetchMore.mockResolvedValue({
      data: { agentSessions: [wireSession("ags-4"), wireSession("ags-5")] },
    });
    const { result, rerender } = renderHook(useCommittedAgentSessions, {
      initialProps: { poll: false, limit: 2 },
    });

    listed = [...listed, wireSession("ags-3")];
    rerender({ poll: false, limit: 2 });
    await act(() => result.current.loadMore());

    expect(fetchMore).toHaveBeenCalledTimes(1);
    expect(fetchMore.mock.calls[0][0].variables).toEqual({ cursor: "ags-3" });
  });

  it("offers more again as soon as a filter change replaces an exhausted list", async () => {
    fetchMore.mockResolvedValue({ data: { agentSessions: [] } });
    const { result, rerender } = renderHook(useCommittedAgentSessions, {
      initialProps: { poll: false, limit: 2 } as UseAgentSessionsOptions,
    });
    expect(result.current.hasMore).toBe(true);

    await act(() => result.current.loadMore());
    expect(result.current.hasMore).toBe(false);

    committedHasMore = [];
    rerender({ poll: false, limit: 2, archived: "archived" });
    expect(result.current.hasMore).toBe(true);
    // The previous filter's end never shows against the new filter's list.
    expect(committedHasMore).toEqual([true]);
  });

  it("offers more again when only the PR-state filter changes", async () => {
    fetchMore.mockResolvedValue({ data: { agentSessions: [] } });
    const { result, rerender } = renderHook(useCommittedAgentSessions, {
      initialProps: { poll: false, limit: 2 } as UseAgentSessionsOptions,
    });
    await act(() => result.current.loadMore());
    expect(result.current.hasMore).toBe(false);

    rerender({ poll: false, limit: 2, prStates: ["merged"] });
    expect(result.current.hasMore).toBe(true);
  });
});

// w9/m386: the list's PR-state filter reaches bex-api as `prStates`.
describe("useAgentSessions PR-state filter", () => {
  beforeEach(() => {
    listed = [];
    queriedVariables = undefined;
  });

  it("queries only the PR states asked for", () => {
    renderHook(() =>
      useAgentSessions({ poll: false, prStates: ["merged", "closed"] }),
    );
    expect(queriedVariables?.prStates).toEqual(["merged", "closed"]);
  });

  it("sends no PR-state filter when none or an empty list is asked for", () => {
    renderHook(() => useAgentSessions({ poll: false }));
    expect(queriedVariables?.prStates).toBeNull();
    renderHook(() => useAgentSessions({ poll: false, prStates: [] }));
    expect(queriedVariables?.prStates).toBeNull();
  });
});
