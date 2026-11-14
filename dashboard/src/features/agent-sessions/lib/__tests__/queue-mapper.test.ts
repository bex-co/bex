import { describe, expect, it } from "vitest";
import {
  toQueuedMessageView,
  toQueuedMessageViews,
  toSteerResult,
} from "@/features/agent-sessions/lib/mapper";

const wireMessage = {
  __typename: "AgentSessionQueuedMessage" as const,
  id: "aqm-1",
  sessionId: "ags-1",
  position: 2,
  prompt: "then the tests",
  author: "kratos-alice",
  attachmentIds: ["att-1", "att-2"],
  blockedReason: "the session's spend ceiling was reached",
  createdAt: "2026-11-14T00:00:00Z",
};

describe("toQueuedMessageView", () => {
  it("projects a queued message", () => {
    expect(toQueuedMessageView(wireMessage)).toEqual({
      id: "aqm-1",
      position: 2,
      prompt: "then the tests",
      author: "kratos-alice",
      attachmentCount: 2,
      blockedReason: "the session's spend ceiling was reached",
      createdAt: "2026-11-14T00:00:00Z",
    });
  });

  it("normalizes absent attachments, blocked reason and time", () => {
    expect(
      toQueuedMessageView({
        ...wireMessage,
        attachmentIds: null,
        blockedReason: null,
        createdAt: null,
      }),
    ).toMatchObject({
      attachmentCount: 0,
      blockedReason: null,
      createdAt: null,
    });
    expect(
      toQueuedMessageView({ ...wireMessage, blockedReason: "" }).blockedReason,
    ).toBeNull();
  });

  it("keeps the server's order for a list, and accepts a missing one", () => {
    expect(
      toQueuedMessageViews([
        { ...wireMessage, id: "aqm-9", position: 1 },
        wireMessage,
      ]).map((m) => m.id),
    ).toEqual(["aqm-9", "aqm-1"]);
    expect(toQueuedMessageViews(null)).toEqual([]);
    expect(toQueuedMessageViews(undefined)).toEqual([]);
  });
});

describe("toSteerResult", () => {
  const session = {
    __typename: "AgentSession" as const,
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

  it("carries the message a steer queued, beside the session's ticket", () => {
    const result = toSteerResult({
      ...session,
      queuedMessage: wireMessage,
    } as never);
    expect(result.session.id).toBe("ags-1");
    expect(result.queuedMessage?.id).toBe("aqm-1");
  });

  it("has no queued message when the steer started a turn", () => {
    expect(
      toSteerResult({ ...session, queuedMessage: null } as never).queuedMessage,
    ).toBeNull();
    expect(toSteerResult(session as never).queuedMessage).toBeNull();
  });
});
