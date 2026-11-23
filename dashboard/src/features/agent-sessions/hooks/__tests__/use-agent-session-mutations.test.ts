import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useAgentSessionMutations } from "@/features/agent-sessions/hooks/use-agent-session-mutations";

const mutate = vi.fn();
const evict = vi.fn();
const gc = vi.fn();
const writeQuery = vi.fn();
const readQuery = vi.fn();

vi.mock("@apollo/client/react", () => ({
  useApolloClient: () => ({ cache: { evict, gc }, writeQuery, readQuery }),
  useMutation: () => [mutate],
}));

describe("useAgentSessionMutations list cache invalidation", () => {
  beforeEach(() => {
    mutate.mockReset();
    evict.mockReset();
    gc.mockReset();
    writeQuery.mockReset();
    readQuery.mockReset();
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
        data: {
          agentSession: { ...session, turnOrigins: null, appliedSecrets: [] },
        },
      }),
    );
    expect(evict).toHaveBeenCalledWith({
      id: "ROOT_QUERY",
      fieldName: "agentSessions",
    });
  });

  it("keeps the detail read's turn origins and applied secrets when a lifecycle result lands (w9/m344, w9/m385)", async () => {
    const turnOrigins = [
      {
        __typename: "AgentSessionTurnOrigin",
        turn: 2,
        kind: "pr_comment",
        author: "octocat",
        url: "https://github.com/acme/widgets/pull/7#issuecomment-1",
      },
    ];
    const appliedSecrets = [
      {
        __typename: "AgentAppliedSecret",
        name: "ORG_TOKEN",
        source: "workspace",
      },
    ];
    readQuery.mockReturnValue({
      agentSession: { id: "ags-1", turnOrigins, appliedSecrets },
    });
    const session = {
      __typename: "AgentSession",
      id: "ags-1",
      phase: "canceled",
    };
    mutate.mockResolvedValue({ data: { cancelAgentSession: session } });
    const { result } = renderHook(() => useAgentSessionMutations());

    await act(() => result.current.cancel("ags-1"));

    expect(writeQuery).toHaveBeenCalledWith(
      expect.objectContaining({
        variables: { id: "ags-1" },
        data: { agentSession: { ...session, turnOrigins, appliedSecrets } },
      }),
    );
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

// w10/m561: the session's own gh binding rides in agentConfig: false turns
// gh off, true turns it on, and an omitted one inherits the workspace's.
describe("useAgentSessionMutations create agentConfig.githubCli", () => {
  beforeEach(() => {
    mutate.mockReset();
    mutate.mockResolvedValue({
      data: {
        createAgentSession: {
          __typename: "AgentSession",
          id: "ags-new",
          phase: "creating",
          ticket: "tkt",
          url: null,
          streamUrl: null,
          expiresAt: null,
        },
      },
    });
  });

  const sentAgentConfig = () =>
    (mutate.mock.calls[0][0] as { variables: { agentConfig: object } })
      .variables.agentConfig;

  it.each([false, true])("sends githubCli %s", async (githubCli) => {
    const { result } = renderHook(() => useAgentSessionMutations());
    await act(() =>
      result.current.create({
        repo: "acme/widgets",
        branch: "bex-agent/fix",
        agent: "claude",
        task: "fix it",
        githubCli,
      }),
    );
    expect(sentAgentConfig()).toMatchObject({ agent: "claude", githubCli });
  });

  it("leaves githubCli out to inherit", async () => {
    const { result } = renderHook(() => useAgentSessionMutations());
    await act(() =>
      result.current.create({
        repo: "acme/widgets",
        branch: "bex-agent/fix",
        agent: "claude",
        task: "fix it",
      }),
    );
    // Undefined is dropped from the request's JSON, so nothing is sent.
    expect(JSON.parse(JSON.stringify(sentAgentConfig()))).not.toHaveProperty(
      "githubCli",
    );
  });
});

// w10/m599: a session's gh binding changes after create through
// updateAgentSession; inherit is resetGithubCli, the null GraphQL cannot
// carry, and the server's view lands in the detail cache.
describe("useAgentSessionMutations setGithubCli", () => {
  const updated = {
    __typename: "AgentSession",
    id: "ags-1",
    ownerId: "tea-1",
    repo: "acme/widgets",
    branch: "bex-agent/fix",
    phase: "running",
    status: "running",
    turns: 1,
    createdAt: "2026-11-23T00:00:00Z",
    updatedAt: "2026-11-23T01:00:00Z",
    agentConfig: {
      __typename: "AgentSessionConfig",
      agent: "claude",
      model: null,
      modelEndpoint: null,
      task: "fix it",
      template: null,
      githubCli: false,
    },
  };

  beforeEach(() => {
    mutate.mockReset();
    writeQuery.mockReset();
    mutate.mockResolvedValue({ data: { updateAgentSession: updated } });
  });

  const sent = () =>
    (mutate.mock.calls[0][0] as { variables: Record<string, unknown> })
      .variables;

  it.each([
    ["off", { id: "ags-1", agentConfig: { githubCli: false } }],
    ["on", { id: "ags-1", agentConfig: { githubCli: true } }],
    ["inherit", { id: "ags-1", resetGithubCli: true }],
  ] as const)("sends %s", async (choice, variables) => {
    const { result } = renderHook(() => useAgentSessionMutations());
    const view = await act(() => result.current.setGithubCli("ags-1", choice));
    expect(sent()).toEqual(variables);
    expect(view.agentConfig.githubCli).toBe(false);
    expect(writeQuery).toHaveBeenCalledWith(
      expect.objectContaining({ variables: { id: "ags-1" } }),
    );
  });

  it("rethrows a refusal as a coded AgentSessionError", async () => {
    const { CombinedGraphQLErrors } = await import("@apollo/client/errors");
    mutate.mockRejectedValueOnce(
      new CombinedGraphQLErrors({
        data: null,
        errors: [
          {
            message: "only a workspace admin chooses or changes gh",
            extensions: {
              code: "AGENT_SESSION_GITHUB_CLI_LOCKED",
              field: "agentConfig.githubCli",
              reason: "admin_required",
            },
          },
        ],
      } as never),
    );
    const { result } = renderHook(() => useAgentSessionMutations());
    await expect(
      act(() => result.current.setGithubCli("ags-1", "off")),
    ).rejects.toMatchObject({
      code: "AGENT_SESSION_GITHUB_CLI_LOCKED",
      params: { reason: "admin_required" },
    });
    expect(writeQuery).not.toHaveBeenCalled();
  });
});
