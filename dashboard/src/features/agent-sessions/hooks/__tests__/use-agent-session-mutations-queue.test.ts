import { act, renderHook } from "@testing-library/react";
import { InMemoryCache } from "@apollo/client";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  AgentSessionQueueDocument,
  type AgentSessionQueuedMessageFieldsFragment,
} from "@/graphql/definitions";
import { useAgentSessionMutations } from "@/features/agent-sessions/hooks/use-agent-session-mutations";

// Stop turn and the queued steer (ADR047 D12) through the session mutations.

const cache = new InMemoryCache();
const client = {
  cache: {
    evict: vi.fn(),
    gc: vi.fn(),
    updateQuery: (...args: Parameters<InMemoryCache["updateQuery"]>) =>
      cache.updateQuery(...args),
  },
  writeQuery: vi.fn(),
  readQuery: vi.fn(),
};
const mutate: Record<string, ReturnType<typeof vi.fn>> = {
  InterruptAgentSession: vi.fn(),
  SteerAgentSession: vi.fn(),
};

vi.mock("@apollo/client/react", () => ({
  useApolloClient: () => client,
  useMutation: (doc: { definitions: Array<{ name?: { value: string } }> }) => [
    mutate[doc.definitions[0].name?.value ?? ""] ?? vi.fn(),
  ],
}));
vi.mock("@/features/usage/context/payment-required-context", () => ({
  usePaymentRequiredGate: () => ({
    run: <T>(fn: () => Promise<T>) => fn(),
  }),
}));

const session = {
  __typename: "AgentSession",
  id: "ags-1",
  ownerId: "tea-1",
  repo: "acme/widgets",
  branch: "bex-agent/fix",
  phase: "running",
  status: "running",
  turns: 2,
  createdAt: "2026-11-14T00:00:00Z",
  updatedAt: "2026-11-14T00:00:00Z",
  ticket: null,
  url: null,
  streamUrl: null,
  expiresAt: null,
};

const queuedWire = (
  id: string,
  position: number,
): AgentSessionQueuedMessageFieldsFragment => ({
  __typename: "AgentSessionQueuedMessage",
  id,
  sessionId: "ags-1",
  position,
  prompt: `prompt ${id}`,
  author: "alice",
  attachmentIds: null,
  blockedReason: null,
  createdAt: null,
});

beforeEach(() => {
  for (const fn of Object.values(mutate)) fn.mockReset();
  client.cache.evict.mockReset();
  client.cache.gc.mockReset();
  client.writeQuery.mockReset();
  client.readQuery.mockReset();
  cache.writeQuery({
    query: AgentSessionQueueDocument,
    variables: { id: "ags-1" },
    data: {
      agentSession: {
        __typename: "AgentSession",
        id: "ags-1",
        pendingMessages: 1,
        queuedMessages: [queuedWire("aqm-1", 1)],
      },
    },
  });
});

const cachedQueue = () =>
  cache
    .readQuery({
      query: AgentSessionQueueDocument,
      variables: { id: "ags-1" },
    })
    ?.agentSession?.queuedMessages?.map((m) => m.id);

describe("interrupt", () => {
  it("fires interruptAgentSession, no-cache, and resolves the session view", async () => {
    mutate.InterruptAgentSession.mockResolvedValue({
      data: { interruptAgentSession: session },
    });
    const { result } = renderHook(() => useAgentSessionMutations());
    const view = await act(() => result.current.interrupt("ags-1"));

    expect(mutate.InterruptAgentSession).toHaveBeenCalledWith(
      expect.objectContaining({
        variables: { id: "ags-1" },
        fetchPolicy: "no-cache",
      }),
    );
    expect(view.id).toBe("ags-1");
    expect(view.phase).toBe("running");
    // Written like Cancel's result: the detail read is primed, lists expire.
    expect(client.writeQuery).toHaveBeenCalled();
    expect(client.cache.evict).toHaveBeenCalledWith({
      id: "ROOT_QUERY",
      fieldName: "agentSessions",
    });
  });

  it("types a coded refusal", async () => {
    const { CombinedGraphQLErrors } = await import("@apollo/client/errors");
    mutate.InterruptAgentSession.mockRejectedValue(
      new CombinedGraphQLErrors({
        data: null,
        errors: [
          {
            message: "idle",
            extensions: { code: "AGENT_SESSION_NO_TURN_RUNNING" },
          },
        ],
      } as never),
    );
    const { result } = renderHook(() => useAgentSessionMutations());
    await expect(
      act(() => result.current.interrupt("ags-1")),
    ).rejects.toMatchObject({ code: "AGENT_SESSION_NO_TURN_RUNNING" });
  });
});

describe("steer", () => {
  it("passes the idempotency key to a send, and returns the message it queued", async () => {
    mutate.SteerAgentSession.mockResolvedValue({
      data: {
        steerAgentSession: {
          ...session,
          queuedMessage: queuedWire("aqm-2", 2),
        },
      },
    });
    const { result } = renderHook(() => useAgentSessionMutations());
    const out = await act(() =>
      result.current.steer("ags-1", "next", { idempotencyKey: "key-1" }),
    );

    expect(mutate.SteerAgentSession).toHaveBeenCalledWith(
      expect.objectContaining({
        variables: expect.objectContaining({
          id: "ags-1",
          prompt: "next",
          idempotencyKey: "key-1",
        }),
      }),
    );
    expect(out.queuedMessage?.id).toBe("aqm-2");
    expect(out.session.id).toBe("ags-1");
  });

  it("shows the queued message in the cached queue at once", async () => {
    mutate.SteerAgentSession.mockResolvedValue({
      data: {
        steerAgentSession: {
          ...session,
          queuedMessage: queuedWire("aqm-2", 2),
        },
      },
    });
    const { result } = renderHook(() => useAgentSessionMutations());
    await act(() => result.current.steer("ags-1", "next"));
    expect(cachedQueue()).toEqual(["aqm-1", "aqm-2"]);
  });

  it("leaves the queue alone when the steer started a turn", async () => {
    mutate.SteerAgentSession.mockResolvedValue({
      data: { steerAgentSession: { ...session, queuedMessage: null } },
    });
    const { result } = renderHook(() => useAgentSessionMutations());
    const out = await act(() => result.current.steer("ags-1", "go"));
    expect(out.queuedMessage).toBeNull();
    expect(cachedQueue()).toEqual(["aqm-1"]);
    expect(mutate.SteerAgentSession).toHaveBeenCalledWith(
      expect.objectContaining({
        variables: expect.objectContaining({ idempotencyKey: undefined }),
      }),
    );
  });
});
