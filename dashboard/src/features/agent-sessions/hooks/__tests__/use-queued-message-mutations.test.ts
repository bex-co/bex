import { act, renderHook } from "@testing-library/react";
import { InMemoryCache } from "@apollo/client";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  AgentSessionQueueDocument,
  type AgentSessionQueuedMessageFieldsFragment,
} from "@/graphql/definitions";
import { useQueuedMessageMutations } from "@/features/agent-sessions/hooks/use-queued-message-mutations";
import { AgentSessionError } from "@/features/agent-sessions/lib/errors";

// The queued-message verbs (ADR047 D12) and "Send now", which composes Stop turn
// and a move to the front.

function wire(
  id: string,
  prompt: string,
  position: number,
): AgentSessionQueuedMessageFieldsFragment {
  return {
    __typename: "AgentSessionQueuedMessage",
    id,
    sessionId: "ags-1",
    position,
    prompt,
    author: "alice",
    attachmentIds: null,
    blockedReason: null,
    createdAt: "2026-11-14T00:00:00Z",
  };
}

const THREE = [wire("a", "one", 1), wire("b", "two", 2), wire("c", "three", 3)];

const cache = new InMemoryCache();
const client = { cache, query: vi.fn() };
const mutate: Record<string, ReturnType<typeof vi.fn>> = {
  EditQueuedAgentSessionMessage: vi.fn(),
  DeleteQueuedAgentSessionMessage: vi.fn(),
  MoveQueuedAgentSessionMessage: vi.fn(),
};
const interrupt = vi.fn();

vi.mock("@apollo/client/react", () => ({
  useApolloClient: () => client,
  useMutation: (doc: { definitions: Array<{ name?: { value: string } }> }) => [
    mutate[doc.definitions[0].name?.value ?? ""],
  ],
}));
vi.mock("@/features/agent-sessions/hooks/use-agent-session-mutations", () => ({
  useAgentSessionMutations: () => ({ interrupt }),
}));

function seedQueue(queue = THREE) {
  cache.writeQuery({
    query: AgentSessionQueueDocument,
    variables: { id: "ags-1" },
    data: {
      agentSession: {
        __typename: "AgentSession",
        id: "ags-1",
        pendingMessages: queue.length,
        queuedMessages: queue,
      },
    },
  });
}
const cachedIds = () =>
  cache
    .readQuery({
      query: AgentSessionQueueDocument,
      variables: { id: "ags-1" },
    })
    ?.agentSession?.queuedMessages?.map((m) => m.id);

const coded = (code: string) => new AgentSessionError(code, "server copy", {});

/** The graphql-level failure a mutation rejects with. */
async function gqlFailure(code: string) {
  const { CombinedGraphQLErrors } = await import("@apollo/client/errors");
  return new CombinedGraphQLErrors({
    data: null,
    errors: [{ message: "server copy", extensions: { code } }],
  } as never);
}

function setup() {
  return renderHook(() => useQueuedMessageMutations("ags-1")).result;
}

beforeEach(() => {
  for (const fn of Object.values(mutate)) fn.mockReset();
  interrupt.mockReset();
  interrupt.mockResolvedValue({ id: "ags-1" });
  client.query.mockReset();
  client.query.mockResolvedValue({});
  seedQueue();
});

describe("edit", () => {
  it("sends the new prompt, shows it at once and reads the queue again", async () => {
    mutate.EditQueuedAgentSessionMessage.mockResolvedValue({
      data: { editQueuedAgentSessionMessage: wire("b", "two, edited", 2) },
    });
    const result = setup();
    const view = await act(() => result.current.edit("b", "two, edited"));

    expect(mutate.EditQueuedAgentSessionMessage).toHaveBeenCalledWith(
      expect.objectContaining({
        variables: { id: "ags-1", messageId: "b", prompt: "two, edited" },
      }),
    );
    expect(view.prompt).toBe("two, edited");
    const cached = cache.readQuery({
      query: AgentSessionQueueDocument,
      variables: { id: "ags-1" },
    });
    expect(cached?.agentSession?.queuedMessages?.[1].prompt).toBe(
      "two, edited",
    );
    expect(client.query).toHaveBeenCalledWith(
      expect.objectContaining({
        query: AgentSessionQueueDocument,
        variables: { id: "ags-1" },
        fetchPolicy: "network-only",
      }),
    );
  });

  it("still reads the queue again when the message already started, and types the error", async () => {
    mutate.EditQueuedAgentSessionMessage.mockRejectedValue(
      await gqlFailure("AGENT_QUEUED_MESSAGE_DISPATCHED"),
    );
    const result = setup();
    await expect(
      act(() => result.current.edit("b", "x")),
    ).rejects.toMatchObject({ code: "AGENT_QUEUED_MESSAGE_DISPATCHED" });
    expect(cachedIds()).toEqual(["a", "b", "c"]);
    expect(client.query).toHaveBeenCalledTimes(1);
  });
});

describe("remove", () => {
  it("removes the message from the cached queue and reads the queue again", async () => {
    mutate.DeleteQueuedAgentSessionMessage.mockResolvedValue({
      data: { deleteQueuedAgentSessionMessage: true },
    });
    const result = setup();
    await act(() => result.current.remove("b"));
    expect(mutate.DeleteQueuedAgentSessionMessage).toHaveBeenCalledWith(
      expect.objectContaining({
        variables: { id: "ags-1", messageId: "b" },
      }),
    );
    expect(cachedIds()).toEqual(["a", "c"]);
    expect(client.query).toHaveBeenCalledTimes(1);
  });

  it("keeps the queue when the server did not confirm", async () => {
    mutate.DeleteQueuedAgentSessionMessage.mockResolvedValue({
      data: { deleteQueuedAgentSessionMessage: false },
    });
    const result = setup();
    await expect(act(() => result.current.remove("b"))).rejects.toThrow();
    expect(cachedIds()).toEqual(["a", "b", "c"]);
  });
});

describe("move", () => {
  it("takes the order the server answers and reads the queue again", async () => {
    mutate.MoveQueuedAgentSessionMessage.mockResolvedValue({
      data: {
        moveQueuedAgentSessionMessage: [
          wire("c", "three", 1),
          wire("a", "one", 2),
          wire("b", "two", 3),
        ],
      },
    });
    const result = setup();
    const views = await act(() => result.current.move("c", 1));
    expect(mutate.MoveQueuedAgentSessionMessage).toHaveBeenCalledWith(
      expect.objectContaining({
        variables: { id: "ags-1", messageId: "c", position: 1 },
      }),
    );
    expect(views.map((m) => m.id)).toEqual(["c", "a", "b"]);
    expect(cachedIds()).toEqual(["c", "a", "b"]);
    expect(client.query).toHaveBeenCalledTimes(1);
  });
});

describe("sendNow", () => {
  beforeEach(() => {
    mutate.MoveQueuedAgentSessionMessage.mockResolvedValue({
      data: {
        moveQueuedAgentSessionMessage: [
          wire("c", "three", 1),
          wire("a", "one", 2),
          wire("b", "two", 3),
        ],
      },
    });
  });

  it("stops the turn first, then moves the message to the front", async () => {
    const result = setup();
    const outcome = await act(() => result.current.sendNow("c", 2));

    expect(outcome).toEqual({ stopped: true });
    expect(interrupt).toHaveBeenCalledWith("ags-1");
    expect(mutate.MoveQueuedAgentSessionMessage).toHaveBeenCalledWith(
      expect.objectContaining({
        variables: { id: "ags-1", messageId: "c", position: 1 },
      }),
    );
    expect(interrupt.mock.invocationCallOrder[0]).toBeLessThan(
      mutate.MoveQueuedAgentSessionMessage.mock.invocationCallOrder[0],
    );
  });

  it("only stops the turn for a message that is already first", async () => {
    const result = setup();
    const outcome = await act(() => result.current.sendNow("a", 0));
    expect(outcome).toEqual({ stopped: true });
    expect(interrupt).toHaveBeenCalledTimes(1);
    expect(mutate.MoveQueuedAgentSessionMessage).not.toHaveBeenCalled();
    // The queue is still read again.
    expect(client.query).toHaveBeenCalledTimes(1);
  });

  it("only moves when no turn is running", async () => {
    interrupt.mockRejectedValue(coded("AGENT_SESSION_NO_TURN_RUNNING"));
    const result = setup();
    const outcome = await act(() => result.current.sendNow("c", 2));
    expect(outcome).toEqual({ stopped: false });
    expect(mutate.MoveQueuedAgentSessionMessage).toHaveBeenCalledTimes(1);
  });

  it("changes nothing while the turn's sandbox is still starting", async () => {
    interrupt.mockRejectedValue(coded("AGENT_SESSION_TURN_STARTING"));
    const result = setup();
    await expect(
      act(() => result.current.sendNow("c", 2)),
    ).rejects.toMatchObject({ code: "AGENT_SESSION_TURN_STARTING" });
    expect(mutate.MoveQueuedAgentSessionMessage).not.toHaveBeenCalled();
    expect(cachedIds()).toEqual(["a", "b", "c"]);
  });

  it.each(["AGENT_SESSION_NOT_STEERABLE", "AGENT_SESSION_NOT_FOUND"])(
    "does not move anything when the stop is refused with %s",
    async (code) => {
      interrupt.mockRejectedValue(coded(code));
      const result = setup();
      await expect(
        act(() => result.current.sendNow("c", 2)),
      ).rejects.toMatchObject({ code });
      expect(mutate.MoveQueuedAgentSessionMessage).not.toHaveBeenCalled();
    },
  );

  it("counts a message that started in the meantime as sent", async () => {
    mutate.MoveQueuedAgentSessionMessage.mockRejectedValue(
      await gqlFailure("AGENT_QUEUED_MESSAGE_DISPATCHED"),
    );
    const result = setup();
    const outcome = await act(() => result.current.sendNow("c", 2));
    expect(outcome).toEqual({ stopped: true });
  });

  it("reports a message that left the queue after the stop", async () => {
    mutate.MoveQueuedAgentSessionMessage.mockRejectedValue(
      await gqlFailure("AGENT_QUEUED_MESSAGE_NOT_FOUND"),
    );
    const result = setup();
    await expect(
      act(() => result.current.sendNow("c", 2)),
    ).rejects.toMatchObject({ code: "AGENT_QUEUED_MESSAGE_NOT_FOUND" });
    expect(interrupt).toHaveBeenCalledTimes(1);
  });
});
