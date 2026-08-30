import { describe, expect, it } from "vitest";
import { deriveConversationState } from "@/features/agent-sessions/lib/conversation-state";

describe("deriveConversationState", () => {
  it.each([
    [{ phase: "creating" as const }, "not-started"],
    [
      {
        phase: "creating" as const,
        resuming: true,
      },
      "connecting",
    ],
    [
      {
        phase: "running" as const,
        transportStatus: "ready",
        hasMessages: true,
      },
      "live",
    ],
    [
      {
        phase: "running" as const,
        transportError: true,
      },
      "broken",
    ],
    [{ phase: "completed" as const }, "ended"],
  ])("maps %o to %s", (input, expected) => {
    expect(deriveConversationState(input)).toBe(expected);
  });

  it("treats an attach race during Creating as connecting, not broken", () => {
    expect(
      deriveConversationState({
        phase: "creating",
        transportError: true,
      }),
    ).toBe("connecting");
  });
});
