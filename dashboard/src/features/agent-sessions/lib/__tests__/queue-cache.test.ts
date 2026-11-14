import { describe, expect, it } from "vitest";
import { InMemoryCache } from "@apollo/client";
import {
  AgentSessionQueueDocument,
  type AgentSessionQueuedMessageFieldsFragment,
} from "@/graphql/definitions";
import {
  patchCachedQueue,
  withEditedMessage,
  withoutQueuedMessage,
  withQueuedMessage,
} from "@/features/agent-sessions/lib/queue-cache";

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

describe("queue list edits", () => {
  it("appends a queued message last", () => {
    const out = withQueuedMessage(THREE, wire("d", "four", 0));
    expect(out.map((m) => [m.id, m.position])).toEqual([
      ["a", 1],
      ["b", 2],
      ["c", 3],
      ["d", 4],
    ]);
  });

  it("does not double a message a retried send returned again", () => {
    const out = withQueuedMessage(THREE, wire("b", "two", 2));
    expect(out.map((m) => m.id)).toEqual(["a", "c", "b"]);
    expect(out.map((m) => m.position)).toEqual([1, 2, 3]);
  });

  it("replaces an edited message in place", () => {
    const out = withEditedMessage(THREE, wire("b", "two, edited", 2));
    expect(out.map((m) => m.prompt)).toEqual(["one", "two, edited", "three"]);
  });

  it("drops a removed message and closes the gap in the places", () => {
    const out = withoutQueuedMessage(THREE, "a");
    expect(out.map((m) => [m.id, m.position])).toEqual([
      ["b", 1],
      ["c", 2],
    ]);
  });
});

describe("patchCachedQueue", () => {
  function cacheWith(queue: AgentSessionQueuedMessageFieldsFragment[]) {
    const cache = new InMemoryCache();
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
    return cache;
  }
  const read = (cache: InMemoryCache) =>
    cache.readQuery({
      query: AgentSessionQueueDocument,
      variables: { id: "ags-1" },
    })?.agentSession;

  it("rewrites the cached queue and keeps the pending count in step", () => {
    const cache = cacheWith(THREE);
    patchCachedQueue(cache, "ags-1", (queue) =>
      withoutQueuedMessage(queue, "b"),
    );
    const session = read(cache);
    expect(session?.queuedMessages?.map((m) => m.id)).toEqual(["a", "c"]);
    expect(session?.pendingMessages).toBe(2);
  });

  it("takes a whole new order, as a move answers it", () => {
    const cache = cacheWith(THREE);
    patchCachedQueue(cache, "ags-1", () => [THREE[2], THREE[0], THREE[1]]);
    expect(read(cache)?.queuedMessages?.map((m) => [m.id, m.position])).toEqual(
      [
        ["c", 1],
        ["a", 2],
        ["b", 3],
      ],
    );
  });

  it("does nothing for a session whose queue was never read", () => {
    const cache = new InMemoryCache();
    patchCachedQueue(cache, "ags-1", (queue) =>
      withQueuedMessage(queue, wire("a", "one", 1)),
    );
    expect(read(cache)).toBeUndefined();
  });

  it("leaves another session's queue alone", () => {
    const cache = cacheWith(THREE);
    patchCachedQueue(cache, "ags-2", () => []);
    expect(read(cache)?.queuedMessages).toHaveLength(3);
  });
});
